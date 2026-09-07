package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateRequiresPostgresOutsideExplicitJSONDev(t *testing.T) {
	cfg := Config{Environment: "development", AllowJSONStore: false}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected DATABASE_URL to be required")
	}
	cfg.AllowJSONStore = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("json store should be allowed in development: %v", err)
	}
}

func TestBackupConnectionDefaultsToAPIOrUsesSeparateRole(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app@localhost/database")
	t.Setenv("BACKUP_DATABASE_URL", "")
	if cfg := Load(); cfg.BackupDatabaseURL != cfg.DatabaseURL {
		t.Fatal("backup fallback differs")
	}
	t.Setenv("BACKUP_DATABASE_URL", "postgres://backup@localhost/database")
	if cfg := Load(); cfg.BackupDatabaseURL != "postgres://backup@localhost/database" {
		t.Fatal("separate backup connection ignored")
	}
}

func TestValidateRejectsJSONInProduction(t *testing.T) {
	cfg := Config{Environment: "production", AllowJSONStore: true, BackupOffsiteDir: `E:\ChakuChuriBackups`, OffsiteRequired: true, AllowSameVolume: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production without DATABASE_URL to fail even if JSON is requested")
	}
	cfg.DatabaseURL = "postgres://chakuchuri:secret@localhost:5432/chakuchuri"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production with postgres and offsite should pass: %v", err)
	}
}

func TestValidateRejectsOffsiteInsideApp(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	cfg := Config{
		Environment:      "production",
		DatabaseURL:      "postgres://chakuchuri:secret@localhost:5432/chakuchuri",
		BackupOffsiteDir: filepath.Join(root, "backups"),
		OffsiteRequired:  true,
		AllowSameVolume:  true,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected offsite inside the app directory to fail")
	}
}

func TestPathInside(t *testing.T) {
	if !pathInside(`D:\app\backups`, `D:\app`) {
		t.Fatal("expected nested path to be inside")
	}
	if pathInside(`E:\backups`, `D:\app`) {
		t.Fatal("did not expect a different root to be inside")
	}
}

func TestFindProjectRootFromBackendAndNestedBinary(t *testing.T) {
	root := t.TempDir()
	backend := filepath.Join(root, "backend")
	frontend := filepath.Join(root, "frontend")
	if err := os.MkdirAll(filepath.Join(root, "scripts", ".bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(frontend, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{backend, filepath.Join(root, "scripts", ".bin")} {
		if got := findProjectRoot(start); got != root {
			t.Fatalf("findProjectRoot(%q) = %q, want %q", start, got, root)
		}
	}
}

func TestValidateLocalDiskRejectsNonDStorage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume enforcement only")
	}
	t.Setenv("CC_LOCAL_DISK_ONLY", "1")
	t.Setenv("CC_LOCAL_DISK", "D:")
	cfg := Config{
		DataDir:       `D:\ChakuChuriData\data`,
		StorageDir:    `D:\ChakuChuriData\storage`,
		BackupDir:     `D:\ChakuChuriData\backups`,
		MigrationsDir: `D:\ChakuChuriData\migrations`,
	}
	if err := cfg.validateLocalDisk(); err != nil {
		t.Fatalf("D: storage should be accepted: %v", err)
	}
	cfg.StorageDir = `C:\ChakuChuriData\storage`
	if err := cfg.validateLocalDisk(); err == nil {
		t.Fatal("expected C: storage to be rejected")
	}
}
