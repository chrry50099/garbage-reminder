package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateFileIfMissingCopiesLegacyData(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "data", "history.db")
	target := filepath.Join(base, "share", "garbage_eta", "history.db")

	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	migrated, err := migrateFileIfMissing(legacy, target)
	if err != nil {
		t.Fatalf("migrateFileIfMissing() returned error: %v", err)
	}
	if !migrated {
		t.Fatal("expected legacy file to be migrated")
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(content) != "legacy-data" {
		t.Fatalf("unexpected migrated content: %s", string(content))
	}
}

func TestMigrateFileIfMissingDoesNotOverrideExistingTarget(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "data", "state.json")
	target := filepath.Join(base, "share", "garbage_eta", "state.json")

	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}
	if err := os.WriteFile(target, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	migrated, err := migrateFileIfMissing(legacy, target)
	if err != nil {
		t.Fatalf("migrateFileIfMissing() returned error: %v", err)
	}
	if migrated {
		t.Fatal("expected migration to be skipped when target exists")
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("expected existing target content to remain, got %s", string(content))
	}
}
