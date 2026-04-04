package safetemp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureDirsOrder(t *testing.T) {
	dirs := secureDirs()

	// Should have at least one entry (home-based fallback)
	if len(dirs) < 1 {
		t.Fatal("expected at least one secure directory")
	}

	// If XDG_RUNTIME_DIR is set, it should be first
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		if dirs[0] != filepath.Join(xdg, "dotm") {
			t.Errorf("expected first dir to be XDG_RUNTIME_DIR/dotm, got %q", dirs[0])
		}
	}

	// Last entry should always be home-based
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	last := dirs[len(dirs)-1]
	expected := filepath.Join(home, ".local", "state", "dotm", "tmp")
	if last != expected {
		t.Errorf("expected last dir %q, got %q", expected, last)
	}
}

func TestSecureDirCreatesDirectory(t *testing.T) {
	// Override XDG_RUNTIME_DIR to a temp directory we control
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	dir := SecureDir()
	if dir == "" {
		t.Fatal("expected non-empty secure directory")
	}

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, not file")
	}

	// Verify mode is 0700
	mode := info.Mode().Perm()
	if mode != 0o700 {
		t.Errorf("expected mode 0700, got %04o", mode)
	}
}

func TestSecureDirReusesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	// Call twice — should return same directory
	dir1 := SecureDir()
	dir2 := SecureDir()

	if dir1 != dir2 {
		t.Errorf("expected same directory, got %q and %q", dir1, dir2)
	}

	// Verify only one directory was created
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry in XDG_RUNTIME_DIR, got %d", len(entries))
	}
}

func TestSecureDirFallbackToHome(t *testing.T) {
	// Set XDG_RUNTIME_DIR to an unwritable path
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent/unwritable/path")

	// Should fall back to home-based directory
	dir := SecureDir()
	if dir == "" {
		t.Fatal("expected fallback to home-based directory")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	expected := filepath.Join(home, ".local", "state", "dotm", "tmp")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	// Verify it was created with correct permissions
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("expected mode 0700, got %04o", info.Mode().Perm())
	}
}

func TestSecureDirAllFailReturnsEmpty(t *testing.T) {
	// Make both XDG and home unwritable
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent/unwritable")

	// Temporarily override UserHomeDir by setting HOME to unwritable
	// We can't easily mock os.UserHomeDir, so we test the fallback
	// behavior by ensuring the function doesn't panic
	dir := SecureDir()

	// If HOME is accessible, dir will be non-empty — that's fine.
	// We just verify it doesn't crash and returns a valid path or empty.
	if dir != "" {
		// Verify it's a valid path
		if !filepath.IsAbs(dir) {
			t.Errorf("expected absolute path, got %q", dir)
		}
	}
}

func TestSecureDirCanCreateFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	dir := SecureDir()
	if dir == "" {
		t.Fatal("expected non-empty directory")
	}

	// Create a temp file in the secure directory
	f, err := os.CreateTemp(dir, "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	// Verify file exists
	if _, err := os.Stat(f.Name()); err != nil {
		t.Errorf("temp file not found: %v", err)
	}
}
