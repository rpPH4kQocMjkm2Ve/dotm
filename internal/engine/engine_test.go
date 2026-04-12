package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotm/internal/config"
	"dotm/internal/manifest"
	"dotm/internal/prompt"
)

// ─── applySymlinks ──────────────────────────────────────────────────────────

func TestApplySymlinksCreates(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/some/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	linkPath := filepath.Join(destDir, "link.txt")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("expected symlink to exist, got %v", err)
	}
	if target != "/some/target" {
		t.Errorf("symlink target = %q, want /some/target", target)
	}
}

func TestApplySymlinksSkipsIfCorrect(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	linkPath := filepath.Join(destDir, "link.txt")
	os.Symlink("/some/target", linkPath)

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/some/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Should not error or change anything.
	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("expected symlink to still exist, got %v", err)
	}
	if target != "/some/target" {
		t.Errorf("symlink target = %q, want /some/target", target)
	}
}

func TestApplySymlinksReplacesWrongTarget(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	linkPath := filepath.Join(destDir, "link.txt")
	os.Symlink("/old/target", linkPath)

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/new/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("expected symlink to exist, got %v", err)
	}
	if target != "/new/target" {
		t.Errorf("symlink target = %q, want /new/target", target)
	}
}

func TestApplySymlinksReplacesRegularFile(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	linkPath := filepath.Join(destDir, "link.txt")
	os.WriteFile(linkPath, []byte("regular file"), 0o644)

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/some/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected path to exist, got %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected a symlink, got regular file/directory")
	}
}

func TestApplySymlinksDryRun(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/some/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(destDir, "link.txt")); err == nil {
		t.Error("dry run should not create symlinks")
	}
}

func TestApplySymlinksTemplateTarget(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{
		Dest:  destDir,
		Shell: "bash",
		Symlinks: map[string]string{
			"config": "{{ .homeDir }}/.config",
		},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.applySymlinks(); err != nil {
		t.Fatalf("applySymlinks: %v", err)
	}

	linkPath := filepath.Join(destDir, "config")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("expected symlink to exist, got %v", err)
	}
	home, _ := os.UserHomeDir()
	want := home + "/.config"
	if target != want {
		t.Errorf("symlink target = %q, want %q", target, want)
	}
}

// ─── Diff ───────────────────────────────────────────────────────────────────

func TestDiffClean(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("same"), 0o644)
	os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("same"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

func TestDiffModified(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("source"), 0o644)
	os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("dest"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

func TestDiffNewFile(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "new.conf"), []byte("new content"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

func TestDiffNoFilesDir(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

func TestDiffTemplateFile(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "greet.txt.tmpl"), []byte("hello {{ .hostname }}"), 0o644)

	hostname, _ := os.Hostname()
	os.WriteFile(filepath.Join(destDir, "greet.txt"), []byte("hello "+hostname), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

func TestDiffIgnoredFile(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.secret"), []byte("source"), 0o644)
	os.WriteFile(filepath.Join(destDir, "test.secret"), []byte("dest"), 0o644)

	// Create ignore.tmpl.
	os.WriteFile(filepath.Join(sourceDir, "ignore.tmpl"), []byte("*.secret\n"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeFiles)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
}

// ─── runScripts ─────────────────────────────────────────────────────────────

func TestRunScriptsAlways(t *testing.T) {
	sourceDir := t.TempDir()
	outFile := filepath.Join(sourceDir, "output.txt")

	bashPath := "/bin/bash"
	if _, err := os.Stat(bashPath); err != nil {
		bashPath = "/usr/bin/bash"
	}
	if _, err := os.Stat(bashPath); err != nil {
		t.Skip("bash not found")
	}

	scriptPath := filepath.Join(sourceDir, "scripts", "test.sh")
	os.MkdirAll(filepath.Dir(scriptPath), 0o755)
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'ran' > "+outFile), 0o644)

	cfg := &config.Config{
		Dest:  "/tmp",
		Shell: bashPath,
		Scripts: []config.ScriptConfig{
			{Path: "scripts/test.sh", Trigger: "always"},
		},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.runScripts(); err != nil {
		t.Fatalf("runScripts: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file, got %v", err)
	}
	if strings.TrimSpace(string(data)) != "ran" {
		t.Errorf("output = %q, want 'ran'", strings.TrimSpace(string(data)))
	}
}

func TestRunScriptsOnChangeSkipsUnchanged(t *testing.T) {
	sourceDir := t.TempDir()
	outFile := filepath.Join(sourceDir, "output.txt")

	bashPath := "/bin/bash"
	if _, err := os.Stat(bashPath); err != nil {
		bashPath = "/usr/bin/bash"
	}
	if _, err := os.Stat(bashPath); err != nil {
		t.Skip("bash not found")
	}

	scriptPath := filepath.Join(sourceDir, "scripts", "on_change.sh")
	os.MkdirAll(filepath.Dir(scriptPath), 0o755)
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'ran' > "+outFile), 0o644)

	cfg := &config.Config{
		Dest:  "/tmp",
		Shell: bashPath,
		Scripts: []config.ScriptConfig{
			{Path: "scripts/on_change.sh", Trigger: "on_change"},
		},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// First run: executes and records hash.
	if err := eng.runScripts(); err != nil {
		t.Fatalf("runScripts (1st): %v", err)
	}
	if _, err := os.ReadFile(outFile); err != nil {
		t.Fatal("first run: script should have run")
	}

	// Remove output and run again — should be skipped.
	os.Remove(outFile)
	eng2, _ := New(cfg, state, sourceDir, false)
	if err := eng2.runScripts(); err != nil {
		t.Fatalf("runScripts (2nd): %v", err)
	}
	if _, err := os.ReadFile(outFile); err == nil {
		t.Error("second run: script should have been skipped")
	}
}

func TestRunScriptsDryRun(t *testing.T) {
	sourceDir := t.TempDir()

	bashPath := "/bin/bash"
	if _, err := os.Stat(bashPath); err != nil {
		bashPath = "/usr/bin/bash"
	}
	if _, err := os.Stat(bashPath); err != nil {
		t.Skip("bash not found")
	}

	scriptPath := filepath.Join(sourceDir, "scripts", "test.sh")
	os.MkdirAll(filepath.Dir(scriptPath), 0o755)
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'should not run'"), 0o644)

	cfg := &config.Config{
		Dest:  "/tmp",
		Shell: bashPath,
		Scripts: []config.ScriptConfig{
			{Path: "scripts/test.sh", Trigger: "always"},
		},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.runScripts(); err != nil {
		t.Fatalf("runScripts: %v", err)
	}
}

func TestRunScriptsEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.runScripts(); err != nil {
		t.Fatalf("runScripts with no scripts: %v", err)
	}
}

// ─── recordManifest ─────────────────────────────────────────────────────────

func TestRecordManifest(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644)

	cfg := &config.Config{
		Dest:  destDir,
		Shell: "bash",
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	written, _ := eng.walkAndWrite()
	eng.recordManifest(written)

	if len(state.Manifest.Files) != 1 {
		t.Errorf("expected 1 file in manifest, got %d", len(state.Manifest.Files))
	}
	if len(state.Manifest.Files) > 0 && state.Manifest.Files[0] != "test.conf" {
		t.Errorf("expected test.conf, got %q", state.Manifest.Files[0])
	}
}

func TestRecordManifestWithSymlinks(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link.txt": "/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	eng.recordManifest(nil)

	if len(state.Manifest.Symlinks) != 1 {
		t.Errorf("expected 1 symlink in manifest, got %d", len(state.Manifest.Symlinks))
	}
}

// ─── Apply with scripts and symlinks together ───────────────────────────────

func TestApplyFullCycle(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create files.
	filesDir := filepath.Join(sourceDir, "files", ".config")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "app.conf"), []byte("setting=value"), 0o644)

	// Create symlink.
	linkTarget := filepath.Join(sourceDir, "target")
	os.WriteFile(linkTarget, []byte("target data"), 0o644)

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"mylink": linkTarget},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.Apply(ScopeFiles); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify file.
	destPath := filepath.Join(destDir, ".config", "app.conf")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
	if string(data) != "setting=value" {
		t.Errorf("content = %q, want 'setting=value'", data)
	}

	// Verify symlink.
	linkPath := filepath.Join(destDir, "mylink")
	if _, err := os.Readlink(linkPath); err != nil {
		t.Errorf("expected symlink to exist, got %v", err)
	}

	// Verify manifest.
	if len(state.Manifest.Files) != 1 {
		t.Errorf("expected 1 file in manifest, got %d", len(state.Manifest.Files))
	}
}

// ─── renderName ─────────────────────────────────────────────────────────────

func TestRenderNamePlain(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	result, ok, err := eng.renderName("git")
	if err != nil {
		t.Fatalf("renderName: %v", err)
	}
	if !ok {
		t.Error("expected ok=true for plain name")
	}
	if result != "git" {
		t.Errorf("result = %q, want 'git'", result)
	}
}

func TestRenderNameTemplate(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"pkg": "zsh"},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	result, ok, err := eng.renderName("{{ .pkg }}")
	if err != nil {
		t.Fatalf("renderName: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if result != "zsh" {
		t.Errorf("result = %q, want 'zsh'", result)
	}
}

func TestRenderNameEmptyResult(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"empty": ""},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	result, ok, err := eng.renderName("{{ .empty }}")
	if err != nil {
		t.Fatalf("renderName: %v", err)
	}
	if ok {
		t.Error("expected ok=false for empty result")
	}
	if result != "" {
		t.Errorf("result = %q, want ''", result)
	}
}

func TestRenderNameInvalidTemplate(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	_, _, err = eng.renderName("{{ .invalid }}")
	// missingkey=error should cause error for missing key.
	if err == nil {
		t.Error("expected error for missing template key")
	}
}

// ─── loadPkgManifest / savePkgManifest ───────────────────────────────────────

func TestLoadPkgManifestEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	m, err := eng.loadPkgManifest()
	if err != nil {
		t.Fatalf("loadPkgManifest: %v", err)
	}
	if len(m.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(m.Packages))
	}
}

func TestSaveAndLoadPkgManifest(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	pkgs := []manifest.PackageEntry{{Name: "git", Manager: "pacman"}}
	svcs := []manifest.ServiceEntry{{Name: "sshd", Manager: "systemd"}}

	if err := eng.savePkgManifest(pkgs, svcs); err != nil {
		t.Fatalf("savePkgManifest: %v", err)
	}

	m, err := eng.loadPkgManifest()
	if err != nil {
		t.Fatalf("loadPkgManifest: %v", err)
	}
	if len(m.Packages) != 1 || m.Packages[0].Name != "git" {
		t.Errorf("expected git package, got %v", m.Packages)
	}
	if len(m.Services) != 1 || m.Services[0].Name != "sshd" {
		t.Errorf("expected sshd service, got %v", m.Services)
	}
}

// ─── Status with ignore ─────────────────────────────────────────────────────

func TestStatusIgnoresFile(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.secret"), []byte("source"), 0o644)

	// Create ignore.tmpl.
	os.WriteFile(filepath.Join(sourceDir, "ignore.tmpl"), []byte("*.secret\n"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	// Ignored file should not appear in entries.
	for _, entry := range report.Entries {
		if entry.RelPath == "test.secret" {
			t.Error("ignored file should not appear in status entries")
		}
	}
}

// ─── Apply dry run prints output ────────────────────────────────────────────

func TestApplyDryRunWithSymlinks(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{
		Dest:     destDir,
		Shell:    "bash",
		Symlinks: map[string]string{"link": "/target"},
	}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Should not error, just print.
	if err := eng.Apply(ScopeFiles); err != nil {
		t.Fatalf("Apply dry run: %v", err)
	}

	// Symlink should not be created.
	if _, err := os.Lstat(filepath.Join(destDir, "link")); err == nil {
		t.Error("dry run should not create symlinks")
	}
}

// ─── Diff with managers (no managers — no pkg/svc diff) ─────────────────────

func TestDiffWithNoManagers(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	err = eng.Diff(ScopeAll)
	if err != nil {
		t.Fatalf("Diff with no managers: %v", err)
	}
}

func TestStatusWithNoManagers(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatalf("Status with no managers: %v", err)
	}
	if report.HasProblems() {
		t.Error("expected no problems with empty config")
	}
}

// ─── Apply full cycle with packages ─────────────────────────────────────────

func TestApplyFullCycleWithPackages(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create files.
	filesDir := filepath.Join(sourceDir, "files", ".config")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "app.conf"), []byte("setting=value"), 0o644)

	// Create mock check script (returns 0 = installed).
	checkScript := filepath.Join(sourceDir, "check.sh")
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

	// Create dotm.toml with packages.
	tomlContent := `
dest = "` + destDir + `"
shell = "bash"

[managers.mock]
check = "` + checkScript + `"
install = "true"
remove = "true"
enable = "true"
disable = "true"

[mock]
packages = ["git"]
`
	tomlPath := filepath.Join(sourceDir, "dotm.toml")
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.Apply(ScopeAll); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify file was created.
	destPath := filepath.Join(destDir, ".config", "app.conf")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("expected file to exist, got %v", err)
	}

	// Verify manifest saved.
	if len(state.Manifest.Files) != 1 {
		t.Errorf("expected 1 file in manifest, got %d", len(state.Manifest.Files))
	}
}

func TestApplyDryRunWithPackages(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create files (same as full cycle test).
	filesDir := filepath.Join(sourceDir, "files", ".config")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "app.conf"), []byte("setting=value"), 0o644)

	checkScript := filepath.Join(sourceDir, "check.sh")
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755)

	tomlContent := `
dest = "` + destDir + `"
shell = "bash"

[managers.mock]
check = "` + checkScript + `"
install = "true"
remove = "true"
enable = "true"
disable = "true"

[mock]
packages = ["new-pkg"]
`
	tomlPath := filepath.Join(sourceDir, "dotm.toml")
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, true) // dryRun = true
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Should not error, just print dry run messages.
	if err := eng.Apply(ScopeAll); err != nil {
		t.Fatalf("Apply dry run: %v", err)
	}

	// Nothing should be created — verify both file and package state.
	destPath := filepath.Join(destDir, ".config", "app.conf")
	if _, err := os.Stat(destPath); err == nil {
		t.Error("dry run should not create files")
	}
}

func TestApplyWithServices(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a sentinel script that touches a file when run.
	sentinel := filepath.Join(sourceDir, "service-enabled.sentinel")
	enableScript := filepath.Join(sourceDir, "enable.sh")
	os.WriteFile(enableScript, []byte("#!/bin/bash\ntouch "+sentinel), 0o755)

	checkScript := filepath.Join(sourceDir, "check.sh")
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755) // not enabled

	tomlContent := `
dest = "` + destDir + `"
shell = "bash"

[managers.mock]
check = "` + checkScript + `"
install = "true"
remove = "true"
enable = "` + enableScript + `"
disable = "true"

[mock]
services = ["sshd"]
`
	tomlPath := filepath.Join(sourceDir, "dotm.toml")
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.Apply(ScopeAll); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify the enable script was executed.
	if _, err := os.Stat(sentinel); err != nil {
		t.Error("expected sentinel file to be created by enable script")
	}
}

// ─── Status verbose ─────────────────────────────────────────────────────────

func TestStatusVerbose(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeFiles, true)
	if err != nil {
		t.Fatalf("Status verbose: %v", err)
	}

	if len(report.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(report.Entries))
	}
	if report.Entries[0].Status != StatusMissing {
		t.Errorf("expected missing status, got %v", report.Entries[0].Status)
	}
}

// ─── Diff verbose ───────────────────────────────────────────────────────────

func TestDiffVerbose(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	filesDir := filepath.Join(sourceDir, "files")
	os.MkdirAll(filesDir, 0o755)
	os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("new content"), 0o644)
	os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("old content"), 0o644)

	cfg := &config.Config{Dest: destDir, Shell: "bash"}
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		err = eng.Diff(ScopeFiles)
		if err != nil {
			t.Errorf("Diff: %v", err)
		}
	})

	// Verify diff output contains both old and new content.
	if !strings.Contains(output, "old content") || !strings.Contains(output, "new content") {
		t.Errorf("expected diff output to contain both old and new content, got: %s", output)
	}
}
