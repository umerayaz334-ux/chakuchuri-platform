package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var migrationFilePattern = regexp.MustCompile(`^([0-9]+)_(.+)\.up\.sql$`)

type Migration struct {
	Version  int
	Name     string
	UpPath   string
	DownPath string
}

type Status struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Applied   bool   `json:"applied"`
	AppliedAt string `json:"appliedAt,omitempty"`
}

func Discover(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]Migration, 0)
	seen := map[int]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		if existing := seen[version]; existing != "" {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, existing, entry.Name())
		}
		name := strings.ReplaceAll(matches[2], "_", " ")
		upPath := filepath.Join(dir, entry.Name())
		downPath := filepath.Join(dir, strings.TrimSuffix(entry.Name(), ".up.sql")+".down.sql")
		if _, err := os.Stat(downPath); err != nil {
			return nil, fmt.Errorf("missing down migration for %s", entry.Name())
		}
		seen[version] = entry.Name()
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			UpPath:   upPath,
			DownPath: downPath,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func Apply(ctx context.Context, db *sql.DB, dir string) ([]Status, error) {
	found, err := Discover(dir)
	if err != nil {
		return nil, err
	}
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	changed := make([]Status, 0)
	for _, migration := range found {
		if _, exists := applied[migration.Version]; exists {
			continue
		}
		if err := execMigration(ctx, db, migration.UpPath, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1, $2, now())", migration.Version, migration.Name)
			return err
		}); err != nil {
			return changed, fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
		changed = append(changed, Status{Version: migration.Version, Name: migration.Name, Applied: true, AppliedAt: time.Now().UTC().Format(time.RFC3339)})
	}

	return changed, nil
}

func Rollback(ctx context.Context, db *sql.DB, dir string, steps int) ([]Status, error) {
	if steps <= 0 {
		steps = 1
	}

	found, err := Discover(dir)
	if err != nil {
		return nil, err
	}
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	byVersion := map[int]Migration{}
	for _, migration := range found {
		byVersion[migration.Version] = migration
	}

	versions := make([]int, 0, len(applied))
	for version := range applied {
		versions = append(versions, version)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))

	changed := make([]Status, 0)
	for _, version := range versions {
		if len(changed) >= steps {
			break
		}
		migration, ok := byVersion[version]
		if !ok {
			return changed, fmt.Errorf("applied migration %d is missing from migration files", version)
		}
		if err := execMigration(ctx, db, migration.DownPath, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = $1", migration.Version)
			return err
		}); err != nil {
			return changed, fmt.Errorf("rollback migration %d: %w", migration.Version, err)
		}
		changed = append(changed, Status{Version: migration.Version, Name: migration.Name, Applied: false})
	}

	return changed, nil
}

func CurrentStatus(ctx context.Context, db *sql.DB, dir string) ([]Status, error) {
	found, err := Discover(dir)
	if err != nil {
		return nil, err
	}
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(found))
	for _, migration := range found {
		appliedAt, ok := applied[migration.Version]
		statuses = append(statuses, Status{
			Version:   migration.Version,
			Name:      migration.Name,
			Applied:   ok,
			AppliedAt: appliedAt,
		})
	}
	return statuses, nil
}

func ensureTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version int PRIMARY KEY,
  name text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT version, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]string{}
	for rows.Next() {
		var version int
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = appliedAt.UTC().Format(time.RFC3339)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func execMigration(ctx context.Context, db *sql.DB, path string, afterSQL func(tx *sql.Tx) error) error {
	sqlBody, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read sql file: %w", err)
	}
	// Editors commonly add a UTF-8 BOM; PostgreSQL treats it as part of the first token.
	sqlBody = []byte(strings.TrimPrefix(string(sqlBody), "\ufeff"))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(sqlBody)); err != nil {
		return fmt.Errorf("execute sql: %w", err)
	}
	if afterSQL != nil {
		if err := afterSQL(tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
