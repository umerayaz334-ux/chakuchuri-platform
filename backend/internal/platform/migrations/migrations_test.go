package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSortsMigrationsAndRequiresRollback(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "000002_second.up.sql", "SELECT 2;")
	writeTestFile(t, dir, "000002_second.down.sql", "SELECT 2;")
	writeTestFile(t, dir, "000001_first.up.sql", "SELECT 1;")
	writeTestFile(t, dir, "000001_first.down.sql", "SELECT 1;")

	found, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(found))
	}
	if found[0].Version != 1 || found[1].Version != 2 {
		t.Fatalf("migrations were not sorted by version: %#v", found)
	}
}

func TestDiscoverRejectsMissingDownMigration(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "000001_first.up.sql", "SELECT 1;")

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("expected missing down migration error")
	}
}

func writeTestFile(t *testing.T, dir string, name string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
