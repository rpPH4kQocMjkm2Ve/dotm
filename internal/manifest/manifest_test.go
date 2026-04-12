package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	orig := &PkgManifest{
		Packages: []PackageEntry{
			{Name: "git", Manager: "pacman"},
			{Name: "zsh", Manager: "pacman"},
		},
		Services: []ServiceEntry{
			{Name: "firewalld", Manager: "systemd"},
			{Name: "waybar", Manager: "systemd-user"},
		},
	}

	if err := Save(dir, orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(loaded.Packages))
	}
	if loaded.Packages[0].Name != "git" {
		t.Errorf("expected git, got %s", loaded.Packages[0].Name)
	}

	if len(loaded.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(loaded.Services))
	}
	if loaded.Services[0].Name != "firewalld" {
		t.Errorf("expected firewalld, got %s", loaded.Services[0].Name)
	}
	if loaded.Services[1].Manager != "systemd-user" {
		t.Errorf("expected systemd-user manager, got %s", loaded.Services[1].Manager)
	}
}

func TestStateDirUsesTestStateDir(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	got, err := stateDir()
	if err != nil {
		t.Fatalf("stateDir: %v", err)
	}
	if got != dir {
		t.Errorf("stateDir = %q, want %q", got, dir)
	}
}

func TestStateFileUsesTestStateDir(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	path, err := stateFile("/some/config/dir")
	if err != nil {
		t.Fatalf("stateFile: %v", err)
	}
	// Path should be in testStateDir.
	if filepath.Dir(path) != dir {
		t.Errorf("stateFile path not in testStateDir: %s", path)
	}
}

func TestLoadWithInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// Create an invalid state file.
	path, _ := stateFile(dir)
	os.WriteFile(path, []byte("{{{invalid toml"), 0o644)

	// Load should handle error gracefully.
	m, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
	if m == nil {
		t.Error("expected non-nil manifest even on error")
	}
}

func TestLoadWithPkgManifestSection(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// Create state file with pkg_manifest section.
	path, _ := stateFile(dir)
	content := `
[pkg_manifest]
packages = [{name = "vim", manager = "apt"}]
services = [{name = "cron", manager = "systemd"}]
`
	os.WriteFile(path, []byte(content), 0o644)

	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(m.Packages) != 1 || m.Packages[0].Name != "vim" {
		t.Errorf("expected vim package, got %v", m.Packages)
	}
	if len(m.Services) != 1 || m.Services[0].Name != "cron" {
		t.Errorf("expected cron service, got %v", m.Services)
	}
}

func TestLoadWithInvalidPkgManifestSection(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	path, _ := stateFile(dir)
	content := `
[pkg_manifest]
packages = "not a table"
`
	os.WriteFile(path, []byte(content), 0o644)

	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should return empty packages.
	if len(m.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(m.Packages))
	}
}

func TestSaveMergesWithExistingState(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// Create existing state with prompt data.
	path, _ := stateFile(dir)
	existingContent := `
[prompt_data]
theme = "dark"

[manifest]
files = ["old.txt"]
`
	os.WriteFile(path, []byte(existingContent), 0o644)

	// Save new pkg manifest.
	newPkgs := &PkgManifest{
		Packages: []PackageEntry{{Name: "git", Manager: "pacman"}},
	}
	if err := Save(dir, newPkgs); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load raw file and check that prompt_data is still there.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !strings.Contains(string(data), "[prompt_data]") {
		t.Error("expected prompt_data to be preserved after Save")
	}
	if !strings.Contains(string(data), "git") {
		t.Error("expected git package in state file")
	}
}

func TestSaveEmptyManifest(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// Save empty manifest.
	if err := Save(dir, &PkgManifest{}); err != nil {
		t.Fatalf("Save empty: %v", err)
	}

	// Load it back.
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Packages) != 0 || len(m.Services) != 0 {
		t.Errorf("expected empty manifest, got pkgs=%d svcs=%d", len(m.Packages), len(m.Services))
	}
}

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(m.Packages))
	}
	if len(m.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(m.Services))
	}
}

func TestStateFile(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	tests := []struct {
		name      string
		configDir string
	}{
		{"absolute", "/home/user/dotfiles"},
		{"different", "/home/user/root_m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := stateFile(tt.configDir)
			if err != nil {
				t.Fatalf("stateFile: %v", err)
			}
			if !filepath.IsAbs(path) {
				t.Errorf("expected absolute path, got %s", path)
			}
		})
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// Write invalid TOML to the state file path.
	path, err := stateFile(dir)
	if err != nil {
		t.Fatalf("stateFile: %v", err)
	}
	if err := os.WriteFile(path, []byte("this is {{{ invalid toml }}}"), 0o644); err != nil {
		t.Fatalf("write invalid toml: %v", err)
	}

	m, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}

	// Must return a non-nil empty manifest, not nil.
	if m == nil {
		t.Fatal("expected non-nil manifest for invalid TOML, got nil")
	}
	if len(m.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(m.Packages))
	}
	if len(m.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(m.Services))
	}
}

// ─── decodePackageEntries / decodeServiceEntries ────────────────────────────

func TestDecodePackageEntries(t *testing.T) {
	// Test []map[string]any
	v1 := []map[string]any{
		{"name": "git", "manager": "pacman"},
		{"name": "zsh", "manager": "pacman"},
	}
	result1 := decodePackageEntries(v1)
	if len(result1) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result1))
	}
	if result1[0].Name != "git" || result1[0].Manager != "pacman" {
		t.Errorf("first entry = %+v, want git/pacman", result1[0])
	}

	// Test []any
	v2 := []any{
		map[string]any{"name": "vim", "manager": "apt"},
		"not a map", // should be skipped
	}
	result2 := decodePackageEntries(v2)
	if len(result2) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result2))
	}
	if result2[0].Name != "vim" {
		t.Errorf("entry = %+v, want vim", result2[0])
	}

	// Test invalid type
	result3 := decodePackageEntries("invalid")
	if len(result3) != 0 {
		t.Errorf("expected 0 entries for invalid type, got %d", len(result3))
	}

	// Test missing fields
	v3 := []map[string]any{
		{"foo": "bar"},
	}
	result4 := decodePackageEntries(v3)
	if len(result4) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result4))
	}
	if result4[0].Name != "" || result4[0].Manager != "" {
		t.Errorf("entry should have empty fields, got %+v", result4[0])
	}
}

func TestDecodeServiceEntries(t *testing.T) {
	// Test []map[string]any
	v1 := []map[string]any{
		{"name": "firewalld", "manager": "systemd"},
	}
	result1 := decodeServiceEntries(v1)
	if len(result1) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result1))
	}
	if result1[0].Name != "firewalld" || result1[0].Manager != "systemd" {
		t.Errorf("entry = %+v, want firewalld/systemd", result1[0])
	}

	// Test []any with mixed types
	v2 := []any{
		map[string]any{"name": "sshd", "manager": "systemd"},
		42, // should be skipped
	}
	result2 := decodeServiceEntries(v2)
	if len(result2) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result2))
	}

	// Test invalid type
	result3 := decodeServiceEntries(123)
	if len(result3) != 0 {
		t.Errorf("expected 0 entries for invalid type, got %d", len(result3))
	}
}

// ─── Save merges with existing state ────────────────────────────────────────

func TestSaveMergesWithExisting(t *testing.T) {
	dir := t.TempDir()
	testStateDir = dir
	t.Cleanup(func() { testStateDir = "" })

	// First save with packages.
	m1 := &PkgManifest{
		Packages: []PackageEntry{{Name: "git", Manager: "pacman"}},
	}
	if err := Save(dir, m1); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Now save with services only (should merge).
	m2 := &PkgManifest{
		Services: []ServiceEntry{{Name: "firewalld", Manager: "systemd"}},
	}
	if err := Save(dir, m2); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load and verify both are present.
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Note: Save overwrites pkg_manifest, so only services should be there.
	if len(loaded.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(loaded.Services))
	}
}
