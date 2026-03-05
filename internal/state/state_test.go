package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func setConfigHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	return tmpDir
}

func mapFromNames(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, name := range names {
		m[name] = true
	}
	return m
}

func TestSaveLoadRoundTripPreservesCollapsedStacks(t *testing.T) {
	setConfigHome(t)

	input := map[string]bool{
		"api":      true,
		"db":       false,
		"frontend": true,
	}

	if err := Save(input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got := Load()
	want := map[string]bool{
		"api":      true,
		"frontend": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadReturnsEmptyMapWhenFileMissing(t *testing.T) {
	setConfigHome(t)

	got := Load()
	if len(got) != 0 {
		t.Fatalf("Load() len = %d, want 0", len(got))
	}
}

func TestLoadReturnsEmptyMapOnCorruptJSON(t *testing.T) {
	setConfigHome(t)

	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := Load()
	if len(got) != 0 {
		t.Fatalf("Load() len = %d, want 0", len(got))
	}
}

func TestSaveCreatesParentDirectories(t *testing.T) {
	setConfigHome(t)

	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath() error = %v", err)
	}
	dir := filepath.Dir(path)

	if err := Save(map[string]bool{"stack-a": true}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%q exists but is not a directory", dir)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
}

func TestSaveFiltersOutFalseEntries(t *testing.T) {
	setConfigHome(t)

	input := map[string]bool{
		"alpha": true,
		"beta":  false,
		"gamma": true,
		"delta": false,
	}

	if err := Save(input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	got := mapFromNames(sf.CollapsedStacks)
	want := map[string]bool{"alpha": true, "gamma": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collapsed_stacks = %#v, want %#v", got, want)
	}
	if got["beta"] || got["delta"] {
		t.Fatalf("collapsed_stacks contains false entries: %#v", got)
	}
}

func TestSaveUsesTempFileAndRename(t *testing.T) {
	setConfigHome(t)

	if err := Save(map[string]bool{"old": true}); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}

	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath() error = %v", err)
	}
	tmp := path + ".tmp"

	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file should not remain after Save(), stat err = %v", err)
	}

	if err := Save(map[string]bool{"new": true}); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file should not remain after second Save(), stat err = %v", err)
	}

	got := Load()
	want := map[string]bool{"new": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() after second Save() = %#v, want %#v", got, want)
	}
}
