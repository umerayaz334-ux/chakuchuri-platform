package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditCountsContentNotNames(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	fixture := map[string]string{
		"one.PNG": "same image", "nested/two.png": "same image",
		"nested/one.PNG": "different", "empty": "",
	}
	for name, content := range fixture {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := run(root, "", "", &output); err != nil {
		t.Fatal(err)
	}
	var report storageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Files != 4 || report.UniqueContents != 3 || report.DuplicateFiles != 1 || report.DuplicateBytes != 10 || report.UniqueBytes != 19 || report.TotalBytes != 29 {
		t.Fatalf("incorrect report: %+v", report)
	}
	if len(report.ByExtension) != 2 || report.ByExtension[1].Extension != ".png" || report.ByExtension[1].Files != 3 {
		t.Fatalf("incorrect extension totals: %+v", report.ByExtension)
	}
	for name, content := range fixture {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != content {
			t.Fatalf("audit modified file %s", name)
		}
	}
}

func TestAuditErrorsDoNotProducePartialReport(t *testing.T) {
	var output bytes.Buffer
	if err := run(filepath.Join(t.TempDir(), "missing"), "", "", &output); err == nil || output.Len() != 0 {
		t.Fatal("missing directory produced a successful report")
	}
	root := t.TempDir()
	if err := run(root, filepath.Join(root, "missing.png"), "message", &output); err == nil || output.Len() != 0 {
		t.Fatal("missing sample produced a successful report")
	}
	if _, err := estimateSample(root, "message"); err == nil {
		t.Fatal("directory accepted as a sample")
	}
	report, err := auditStorage(root)
	if err != nil || report.Files != 0 || report.TotalBytes != 0 {
		t.Fatalf("empty directory: %+v %v", report, err)
	}
}

func TestAuditDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	report, err := auditStorage(root)
	if err != nil || report.Files != 0 || report.SkippedEntries != 1 {
		t.Fatalf("followed symlink: %+v %v", report, err)
	}
}
