package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

type testSnapshot struct {
	Version int      `json:"version"`
	Rows    []string `json:"rows"`
}

func TestSaveJSONWritesPrimaryAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	if err := SaveJSON(path, testSnapshot{Version: 1, Rows: []string{"first"}}); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if err := SaveJSON(path, testSnapshot{Version: 1, Rows: []string{"second"}}); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var primary testSnapshot
	if _, err := LoadJSON(path, &primary); err != nil {
		t.Fatalf("load primary failed: %v", err)
	}
	if primary.Rows[0] != "second" {
		t.Fatalf("expected latest snapshot, got %#v", primary)
	}

	var backup testSnapshot
	if _, err := LoadJSON(path+".bak", &backup); err != nil {
		t.Fatalf("load backup failed: %v", err)
	}
	if backup.Rows[0] != "first" {
		t.Fatalf("expected previous snapshot in backup, got %#v", backup)
	}
}

func TestLoadJSONFallsBackToBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveJSON(path, testSnapshot{Version: 1, Rows: []string{"healthy"}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if err := SaveJSON(path, testSnapshot{Version: 1, Rows: []string{"latest"}}); err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatalf("corrupt primary failed: %v", err)
	}

	var loaded testSnapshot
	source, err := LoadJSON(path, &loaded)
	if err != nil {
		t.Fatalf("load with fallback failed: %v", err)
	}
	if source != path+".bak" {
		t.Fatalf("expected backup source, got %s", source)
	}
	if loaded.Rows[0] != "healthy" {
		t.Fatalf("expected backup data, got %#v", loaded)
	}
}
