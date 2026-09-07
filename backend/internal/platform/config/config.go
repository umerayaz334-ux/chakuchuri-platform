package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host              string
	Port              string
	AllowedOrigin     string
	Environment       string
	DataDir           string
	DatabaseURL       string
	BackupDatabaseURL string
	MigrationsDir     string
	StorageDir        string
	BackupDir         string
	BackupOffsiteDir  string
	BackupRetention   int
	BackupInterval    time.Duration
	AllowJSONStore    bool
	OffsiteRequired   bool
	AllowSameVolume   bool
	DBMaxOpenConns    int
	DBMaxIdleConns    int
}

func Load() Config {
	root := projectRoot()
	environment := value("APP_ENV", "development")
	allowJSON := boolValue("CC_ALLOW_JSON_STORE", false) && !isProduction(environment)
	offsiteDir := strings.TrimSpace(value("BACKUP_OFFSITE_DIR", ""))
	return Config{
		Host:              value("HOST", "0.0.0.0"),
		Port:              value("PORT", "8002"),
		AllowedOrigin:     value("ALLOWED_ORIGIN", "http://127.0.0.1:5170"),
		Environment:       environment,
		DataDir:           projectPath(root, "DATA_DIR", "backend", "data"),
		DatabaseURL:       strings.TrimSpace(value("DATABASE_URL", "")),
		BackupDatabaseURL: strings.TrimSpace(value("BACKUP_DATABASE_URL", value("DATABASE_URL", ""))),
		MigrationsDir:     projectPath(root, "MIGRATIONS_DIR", "backend", "migrations"),
		StorageDir:        projectPath(root, "STORAGE_DIR", "backend", "storage"),
		BackupDir:         projectPath(root, "BACKUP_DIR", "backend", "backups"),
		BackupOffsiteDir:  offsiteDir,
		BackupRetention:   intValue("BACKUP_RETENTION", 14),
		BackupInterval:    durationValue("BACKUP_INTERVAL", 24*time.Hour),
		AllowJSONStore:    allowJSON,
		OffsiteRequired:   isProduction(environment) || boolValue("BACKUP_OFFSITE_REQUIRED", false) || offsiteDir != "",
		AllowSameVolume:   boolValue("BACKUP_ALLOW_SAME_VOLUME", false),
		DBMaxOpenConns:    boundedIntValue("DB_MAX_OPEN_CONNS", 20, 1, 500),
		DBMaxIdleConns:    boundedIntValue("DB_MAX_IDLE_CONNS", 5, 0, 100),
	}
}

func projectRoot() string {
	if configured := strings.TrimSpace(os.Getenv("APP_ROOT")); configured != "" {
		if absolute, err := filepath.Abs(configured); err == nil {
			return absolute
		}
	}
	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	for _, start := range starts {
		if root := findProjectRoot(start); root != "" {
			return root
		}
	}
	if len(starts) > 0 {
		return starts[0]
	}
	return "."
}

func findProjectRoot(start string) string {
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if directoryExists(filepath.Join(current, "backend")) && directoryExists(filepath.Join(current, "frontend")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func projectPath(root string, key string, parts ...string) string {
	if configured := strings.TrimSpace(os.Getenv(key)); configured != "" {
		if filepath.IsAbs(configured) {
			return filepath.Clean(configured)
		}
		return filepath.Join(root, configured)
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

func (cfg Config) Production() bool {
	return isProduction(cfg.Environment)
}

func (cfg Config) StorageEngine() string {
	if cfg.DatabaseURL != "" {
		return "postgres"
	}
	return "json"
}

func (cfg Config) Validate() error {
	if err := cfg.validateLocalDisk(); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		if cfg.Production() {
			return fmt.Errorf("DATABASE_URL is required in production; JSON files are not the production store")
		}
		if !cfg.AllowJSONStore {
			return fmt.Errorf("DATABASE_URL is required. Start PostgreSQL, or set CC_ALLOW_JSON_STORE=1 for local JSON-only development")
		}
	}
	if !cfg.OffsiteRequired {
		return nil
	}
	if cfg.BackupOffsiteDir == "" {
		return fmt.Errorf("BACKUP_OFFSITE_DIR is required so backups are stored off this application disk")
	}
	absOffsite, err := filepath.Abs(cfg.BackupOffsiteDir)
	if err != nil {
		return fmt.Errorf("BACKUP_OFFSITE_DIR: %w", err)
	}
	cwd, _ := os.Getwd()
	if cwd != "" && pathInside(absOffsite, cwd) {
		return fmt.Errorf("BACKUP_OFFSITE_DIR must not be inside the application directory")
	}
	if cfg.Production() && !cfg.AllowSameVolume && cwd != "" && sameVolume(absOffsite, cwd) {
		return fmt.Errorf("BACKUP_OFFSITE_DIR must be on another disk than the app (set BACKUP_ALLOW_SAME_VOLUME=1 only for a single-disk host with cloud sync)")
	}
	return nil
}

func (cfg Config) validateLocalDisk() error {
	if !boolValue("CC_LOCAL_DISK_ONLY", false) || runtime.GOOS != "windows" {
		return nil
	}
	expected := strings.TrimSpace(value("CC_LOCAL_DISK", "D:"))
	if expected == "" {
		return nil
	}
	expected = strings.TrimRight(strings.ToUpper(filepath.VolumeName(expected)), `\/`)
	if expected == "" {
		return fmt.Errorf("CC_LOCAL_DISK must be a Windows drive such as D:")
	}
	for name, path := range map[string]string{
		"APP_ROOT":       projectRoot(),
		"DATA_DIR":       cfg.DataDir,
		"STORAGE_DIR":    cfg.StorageDir,
		"BACKUP_DIR":     cfg.BackupDir,
		"MIGRATIONS_DIR": cfg.MigrationsDir,
	} {
		volume := strings.ToUpper(filepath.VolumeName(path))
		if volume == "" {
			if absolute, err := filepath.Abs(path); err == nil {
				volume = strings.ToUpper(filepath.VolumeName(absolute))
			}
		}
		if volume != expected {
			return fmt.Errorf("%s must be stored on %s when CC_LOCAL_DISK_ONLY=1 (got %s)", name, expected, path)
		}
	}
	return nil
}
func value(key string, fallback string) string {
	if found := os.Getenv(key); found != "" {
		return found
	}
	return fallback
}

func intValue(key string, fallback int) int {
	parsed, err := strconv.Atoi(os.Getenv(key))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func boundedIntValue(key string, fallback, minimum, maximum int) int {
	parsed := intValue(key, fallback)
	if parsed < minimum {
		return minimum
	}
	if parsed > maximum {
		return maximum
	}
	return parsed
}

func durationValue(key string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(os.Getenv(key))
	if err != nil || parsed < time.Minute {
		return fallback
	}
	return parsed
}

func boolValue(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes"
}

func isProduction(environment string) bool {
	return strings.EqualFold(strings.TrimSpace(environment), "production")
}

func sameVolume(left string, right string) bool {
	leftVol := filepath.VolumeName(left)
	rightVol := filepath.VolumeName(right)
	if leftVol == "" || rightVol == "" {
		return false
	}
	return strings.EqualFold(leftVol, rightVol)
}

func pathInside(child string, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}
