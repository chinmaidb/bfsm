package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationFilesSortsSQLFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"002_second.sql", "001_first.sql", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files, err := migrationFiles(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(directory, "001_first.sql"),
		filepath.Join(directory, "002_second.sql"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("migrationFiles() = %v, want %v", files, want)
	}
}

func TestMigrationFilesRejectsEmptyDirectory(t *testing.T) {
	_, err := migrationFiles(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no SQL migration files found") {
		t.Fatalf("migrationFiles() error = %v, want an empty-migrations error", err)
	}
}
