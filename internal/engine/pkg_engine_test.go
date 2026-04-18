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

// ─── diffPackages ───────────────────────────────────────────────────────────

func TestDiffPackagesInstall(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	} // not installed

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["new-pkg"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffPackages()
	})

	if !strings.Contains(output, "+ install") || !strings.Contains(output, "new-pkg") {
		t.Errorf("expected '+ install new-pkg' in output, got: %s", output)
	}
}

func TestDiffPackagesRemove(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	} // installed

	// Save a manifest with an old package.
	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old-pkg", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevPkgs)

	// No desired packages in config.
	tomlContent := pkgManagerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffPackages()
	})

	if !strings.Contains(output, "- remove") || !strings.Contains(output, "old-pkg") {
		t.Errorf("expected '- remove old-pkg' in output, got: %s", output)
	}
}

func TestDiffPackagesNoChanges(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["git"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffPackages()
	})

	// Should not print anything if no changes.
	if strings.Contains(output, "install") || strings.Contains(output, "remove") {
		t.Errorf("expected no changes in output, got: %s", output)
	}
}

// ─── diffServices ───────────────────────────────────────────────────────────

func TestDiffServicesEnable(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	} // not enabled

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["new-svc"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffServices()
	})

	if !strings.Contains(output, "+ enable") || !strings.Contains(output, "new-svc") {
		t.Errorf("expected '+ enable new-svc' in output, got: %s", output)
	}
}

func TestDiffServicesDisable(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	} // enabled

	prevSvcs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevSvcs)

	tomlContent := svcManagerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffServices()
	})

	if !strings.Contains(output, "- disable") || !strings.Contains(output, "old-svc") {
		t.Errorf("expected '- disable old-svc' in output, got: %s", output)
	}
}

// ─── applyPackages ──────────────────────────────────────────────────────────

func TestApplyPackagesDryRun(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["new-pkg"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true) // dryRun = true
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(true)
		if len(errs) > 0 {
			t.Errorf("applyPackages errors: %v", errs)
		}
		_ = pkgEntries
	})

	if !strings.Contains(output, "[DRY RUN]") || !strings.Contains(output, "new-pkg") {
		t.Errorf("expected [DRY RUN] new-pkg in output, got: %s", output)
	}
}

func TestApplyPackagesAlreadyInstalled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["git"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(false)
		if len(errs) > 0 {
			t.Errorf("applyPackages errors: %v", errs)
		}
		if len(pkgEntries) != 1 || pkgEntries[0].Name != "git" {
			t.Errorf("expected git in pkgEntries, got %v", pkgEntries)
		}
	})

	if strings.Contains(output, "Installed") {
		t.Errorf("should not install already-installed package, got: %s", output)
	}
}

func TestApplyPackagesRemoveObsolete(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old-pkg", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevPkgs)

	tomlContent := managerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(false)
		if len(errs) > 0 {
			t.Errorf("applyPackages errors: %v", errs)
		}
		// Should return empty packages (old removed), no desired packages.
		_ = pkgEntries
	})

	if !strings.Contains(output, "Removed") || !strings.Contains(output, "old-pkg") {
		t.Errorf("expected 'Removed old-pkg' in output, got: %s", output)
	}
}

func TestApplyPackagesRemoveObsoleteDryRun(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old-pkg", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevPkgs)

	tomlContent := managerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(true)
		if len(errs) > 0 {
			t.Errorf("applyPackages errors: %v", errs)
		}
		_ = pkgEntries
	})

	if !strings.Contains(output, "[DRY RUN]") || !strings.Contains(output, "old-pkg") {
		t.Errorf("expected [DRY RUN] old-pkg in output, got: %s", output)
	}
}

// ─── applyServices ──────────────────────────────────────────────────────────

func TestApplyServicesDryRun(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["new-svc"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(true)
		if len(errs) > 0 {
			t.Errorf("applyServices errors: %v", errs)
		}
		_ = svcEntries
	})

	if !strings.Contains(output, "[DRY RUN]") || !strings.Contains(output, "new-svc") {
		t.Errorf("expected [DRY RUN] new-svc in output, got: %s", output)
	}
}

func TestApplyServicesAlreadyEnabled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["sshd"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(false)
		if len(errs) > 0 {
			t.Errorf("applyServices errors: %v", errs)
		}
		if len(svcEntries) != 1 || svcEntries[0].Name != "sshd" {
			t.Errorf("expected sshd in svcEntries, got %v", svcEntries)
		}
	})

	if strings.Contains(output, "Enabled") {
		t.Errorf("should not enable already-enabled service, got: %s", output)
	}
}

func TestApplyServicesDisableObsolete(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevSvcs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevSvcs)

	tomlContent := managerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(false)
		if len(errs) > 0 {
			t.Errorf("applyServices errors: %v", errs)
		}
		_ = svcEntries
	})

	if !strings.Contains(output, "Disabled") || !strings.Contains(output, "old-svc") {
		t.Errorf("expected 'Disabled old-svc' in output, got: %s", output)
	}
}

func TestApplyServicesDisableObsoleteDryRun(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevSvcs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevSvcs)

	tomlContent := managerTOML(checkScript)
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(true)
		if len(errs) > 0 {
			t.Errorf("applyServices errors: %v", errs)
		}
		_ = svcEntries
	})

	if !strings.Contains(output, "[DRY RUN]") || !strings.Contains(output, "old-svc") {
		t.Errorf("expected [DRY RUN] old-svc in output, got: %s", output)
	}
}

func savePkgManifest(t *testing.T, sourceDir string, m *manifest.PkgManifest) {
	t.Helper()
	if err := manifest.Save(sourceDir, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// ─── shellQuote ─────────────────────────────────────────────────────────────

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", "'hello'"},
		{"empty", "", "''"},
		{"with single quote", "it's", "'it'\\''s'"},
		{"with spaces", "hello world", "'hello world'"},
		{"multiple quotes", "a'b'c", "'a'\\''b'\\''c'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── rawData / shellData ────────────────────────────────────────────────────

func TestRawData(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"custom": "value"},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	data := eng.rawData("test-name")

	if data["Name"] != "test-name" {
		t.Errorf("Name = %q, want 'test-name'", data["Name"])
	}
	if data["custom"] != "value" {
		t.Errorf("custom = %q, want 'value'", data["custom"])
	}
}

func TestShellData(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"custom": "value"},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	data := eng.shellData("test-name")

	// Name should be shell-quoted (single-quoted).
	if data["Name"] != "'test-name'" {
		t.Errorf("Name = %q, want \"'test-name'\"", data["Name"])
	}
	if data["custom"] != "value" {
		t.Errorf("custom = %q, want 'value'", data["custom"])
	}
}

// ─── renderCached ───────────────────────────────────────────────────────────

func TestRenderCachedCaches(t *testing.T) {
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

	// First render — should parse and cache.
	result1, err := eng.renderCached("echo {{ .Name }}", eng.shellData("test"))
	if err != nil {
		t.Fatalf("first render: %v", err)
	}

	// Second render — should use cache.
	result2, err := eng.renderCached("echo {{ .Name }}", eng.shellData("test"))
	if err != nil {
		t.Fatalf("second render: %v", err)
	}

	if result1 != result2 {
		t.Errorf("cached results should match: %q != %q", result1, result2)
	}

	// Verify cache is populated.
	if len(eng.tmplCache) != 1 {
		t.Errorf("expected 1 cached template, got %d", len(eng.tmplCache))
	}
}

func TestRenderCachedInvalidTemplate(t *testing.T) {
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

	_, err = eng.renderCached("{{ .invalid }}", eng.shellData("test"))
	if err == nil {
		t.Error("expected error for missing key")
	}
}

// ─── renderName caching behavior ────────────────────────────────────────────

func TestRenderNameCaching(t *testing.T) {
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

	// Render same name twice — should use cache.
	_, _, err = eng.renderNames("{{ .Name }}")
	if err != nil {
		t.Fatalf("first renderNames: %v", err)
	}

	_, _, err = eng.renderNames("{{ .Name }}")
	if err != nil {
		t.Fatalf("second renderNames: %v", err)
	}
}

// ─── check() error path ─────────────────────────────────────────────────────

func TestCheckExecutionError(t *testing.T) {
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

	// Use an invalid bash command that causes execution error (not exit code).
	// Using a command that doesn't exist in $PATH — bash -c will fail with exit 127,
	// which is treated as "not installed", not an execution error.
	// To trigger execution error, we need something like an invalid command structure.
	// Note: In practice, exec.Command("bash", "-c", "...").Run() only returns
	// non-ExitError for things like fork failures, which are hard to trigger in tests.
	// We test the exit code path instead.
	installed, err := eng.check("/nonexistent/command/that/does/not/exist {{.Name}}", "test")
	if err != nil {
		// Execution error — acceptable.
		t.Logf("check returned execution error (acceptable): %v", err)
	} else if installed {
		t.Fatal("check should not return installed=true for nonexistent command")
	}
}

func TestCheckReturnsTrueForZeroExit(t *testing.T) {
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

	installed, err := eng.check("true", "test")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !installed {
		t.Error("expected installed=true for zero exit code")
	}
}

func TestCheckReturnsFalseForNonZeroExit(t *testing.T) {
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

	installed, err := eng.check("false", "test")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if installed {
		t.Error("expected installed=false for non-zero exit code")
	}
}

// ─── applyPackages — error paths ────────────────────────────────────────────

func TestApplyPackagesRenderError(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["{{ .missing_pkg }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		pkgEntries, errs := eng.applyPackages(false)
		// Render error is logged to stderr, not returned as error.
		if len(errs) != 0 {
			t.Logf("applyPackages returned errors (expected): %v", errs)
		}
		_ = pkgEntries
	})

	// Should print WARN about render error.
	if !strings.Contains(output, "WARN") || !strings.Contains(output, "render") {
		t.Errorf("WARN about render error not found in stderr output: %s", output)
	}
}

func TestApplyPackagesDryRunNoInstall(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["new-pkg"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(true)
		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		_ = pkgEntries
	})

	// Should print DRY RUN message but not actually install.
	if !strings.Contains(output, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %s", output)
	}
	if strings.Contains(output, "Installed") {
		t.Error("should not install in dry run mode")
	}
}

func TestApplyPackagesObsoleteManagerMissing(t *testing.T) {
	sourceDir := t.TempDir()

	// Save manifest with obsolete package using non-existent manager.
	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old", Manager: "unknown_mgr"}},
	}
	savePkgManifest(t, sourceDir, prevPkgs)

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
install = "true"
remove = "true"
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		pkgEntries, errs := eng.applyPackages(false)
		_ = errs
		_ = pkgEntries
	})

	if !strings.Contains(output, "WARN") || !strings.Contains(output, "manager") {
		t.Errorf("expected WARN about missing manager, got: %s", output)
	}
}

func TestApplyPackagesObsoleteCheckError(t *testing.T) {
	sourceDir := t.TempDir()

	// Save manifest with obsolete package.
	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevPkgs)

	// Check command returns exit 1 — treated as "not installed".
	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "false"
install = "true"
remove = "true"
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(false)
		_ = errs
		_ = pkgEntries
	})

	// Check returns exit 1 — treated as "not installed", so no removal needed.
	// Verify that nothing is removed (obsolete not-installed packages are skipped).
	if strings.Contains(output, "Removed") {
		t.Errorf("should not remove obsolete package that is not installed, got: %s", output)
	}
}

func TestApplyPackagesEmptyTemplateRender(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = ["{{ .empty_pkg }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["empty_pkg"] = ""

	output := captureStdout(func() {
		pkgEntries, errs := eng.applyPackages(false)
		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		// Empty package should be skipped.
		if len(pkgEntries) != 0 {
			t.Errorf("expected 0 pkg entries for empty template, got %d", len(pkgEntries))
		}
	})

	// Should not print anything since package is empty.
	if strings.Contains(output, "check") || strings.Contains(output, "install") {
		t.Errorf("should not process empty package, got: %s", output)
	}
}

// ─── applyServices — error paths ────────────────────────────────────────────

func TestApplyServicesRenderError(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["{{ .missing_svc }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		svcEntries, errs := eng.applyServices(false)
		// Render error is logged to stderr, not returned as error.
		if len(errs) != 0 {
			t.Logf("applyServices returned errors (expected): %v", errs)
		}
		_ = svcEntries
	})

	if !strings.Contains(output, "WARN") || !strings.Contains(output, "render") {
		t.Errorf("WARN about render error not found in stderr output: %s", output)
	}
}

func TestApplyServicesDryRunNoEnable(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["new-svc"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)

	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, true)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(true)
		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		_ = svcEntries
	})

	if !strings.Contains(output, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %s", output)
	}
	if strings.Contains(output, "Enabled") {
		t.Error("should not enable in dry run mode")
	}
}

func TestApplyServicesObsoleteManagerMissing(t *testing.T) {
	sourceDir := t.TempDir()

	prevSvcs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "unknown_mgr"}},
	}
	savePkgManifest(t, sourceDir, prevSvcs)

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
enable = "true"
disable = "true"
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		svcEntries, errs := eng.applyServices(false)
		_ = errs
		_ = svcEntries
	})

	if !strings.Contains(output, "WARN") || !strings.Contains(output, "manager") {
		t.Errorf("expected WARN about missing manager, got: %s", output)
	}
}

func TestApplyServicesObsoleteCheckError(t *testing.T) {
	sourceDir := t.TempDir()

	prevSvcs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "mock"}},
	}
	savePkgManifest(t, sourceDir, prevSvcs)

	// Check returns exit 1 — treated as "not enabled".
	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "false"
enable = "true"
disable = "true"
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(false)
		_ = errs
		_ = svcEntries
	})

	// Check returns exit 1 — treated as "not enabled", so no disable needed.
	// Verify that nothing is disabled (obsolete not-enabled services are skipped).
	if strings.Contains(output, "Disabled") {
		t.Errorf("should not disable obsolete service that is not enabled, got: %s", output)
	}
}

func TestApplyServicesEmptyTemplateRender(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := svcManagerTOML(checkScript) + `
[mock]
services = ["{{ .empty_svc }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["empty_svc"] = ""

	output := captureStdout(func() {
		svcEntries, errs := eng.applyServices(false)
		if len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		if len(svcEntries) != 0 {
			t.Errorf("expected 0 svc entries for empty template, got %d", len(svcEntries))
		}
	})

	if strings.Contains(output, "check") || strings.Contains(output, "enable") {
		t.Errorf("should not process empty service, got: %s", output)
	}
}

// ─── diffPackages/diffServices — error paths ────────────────────────────────

func TestDiffPackagesRenderError(t *testing.T) {
	sourceDir := t.TempDir()

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
install = "true"
remove = "true"

[mock]
packages = ["{{ .missing }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		eng.diffPackages()
	})

	if !strings.Contains(output, "WARN") || !strings.Contains(output, "render") {
		t.Errorf("expected WARN about render error, got: %s", output)
	}
}

func TestDiffPackagesCheckError(t *testing.T) {
	sourceDir := t.TempDir()

	// Check returns exit 1 — treated as "not installed".
	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "false"
install = "true"
remove = "true"

[mock]
packages = ["test"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffPackages()
	})

	// Should show + install since check returns false.
	if !strings.Contains(output, "+ install") {
		t.Errorf("expected + install, got: %s", output)
	}
}

func TestDiffServicesRenderError(t *testing.T) {
	sourceDir := t.TempDir()

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
enable = "true"
disable = "true"

[mock]
services = ["{{ .missing }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		eng.diffServices()
	})

	if !strings.Contains(output, "WARN") || !strings.Contains(output, "render") {
		t.Errorf("expected WARN about render error, got: %s", output)
	}
}

func TestDiffServicesCheckError(t *testing.T) {
	sourceDir := t.TempDir()

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "false"
enable = "true"
disable = "true"

[mock]
services = ["test"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		eng.diffServices()
	})

	// Should show + enable since check returns false.
	if !strings.Contains(output, "+ enable") {
		t.Errorf("expected + enable, got: %s", output)
	}
}

func TestDiffPackagesEmptyTemplate(t *testing.T) {
	sourceDir := t.TempDir()

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
install = "true"
remove = "true"

[mock]
packages = ["{{ .pkg }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["pkg"] = ""

	output := captureStdout(func() {
		eng.diffPackages()
	})

	// Should not print anything since package renders empty.
	if strings.Contains(output, "install") || strings.Contains(output, "remove") {
		t.Errorf("should not print anything for empty package, got: %s", output)
	}
}

func TestDiffServicesEmptyTemplate(t *testing.T) {
	sourceDir := t.TempDir()

	tomlContent := `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "true"
enable = "true"
disable = "true"

[mock]
services = ["{{ .svc }}"]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["svc"] = ""

	output := captureStdout(func() {
		eng.diffServices()
	})

	// Should not print anything since service renders empty.
	if strings.Contains(output, "enable") || strings.Contains(output, "disable") {
		t.Errorf("should not print anything for empty service, got: %s", output)
	}
}

// ─── Multi-line templates ───────────────────────────────────────────────────

func TestRenderNamesMultiLine(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"laptop": true},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Multi-line template with conditional.
	tmpl := "{{ if .laptop }}\nbrightnessctl\nopentabletdriver\n{{ end }}"
	result, ok, err := eng.renderNames(tmpl)
	if err != nil {
		t.Fatalf("renderNames: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for multi-line result")
	}
	if len(result) != 2 {
		t.Errorf("got %d names, want 2: %v", len(result), result)
	}
	if result[0] != "brightnessctl" || result[1] != "opentabletdriver" {
		t.Errorf("unexpected names: %v", result)
	}
}

func TestRenderNamesMultiLineWithEmptyLines(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash"}
	state := &prompt.State{
		Data:         map[string]any{"laptop": true},
		ScriptHashes: make(map[string]string),
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Template with extra newlines.
	tmpl := "{{ if .laptop }}\n\nbrightnessctl\n\nopentabletdriver\n\n{{ end }}"
	result, ok, err := eng.renderNames(tmpl)
	if err != nil {
		t.Fatalf("renderNames: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(result) != 2 {
		t.Errorf("got %d names, want 2: %v", len(result), result)
	}
}

func TestDiffPackagesMultiLine(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = [
   """
   {{ if .laptop }}
   brightnessctl
   opentabletdriver
   {{ end }}
   """
]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["laptop"] = true

	output := captureStdout(func() {
		eng.diffPackages()
	})

	// Should show both packages as needing install.
	if !strings.Contains(output, "brightnessctl") {
		t.Errorf("expected brightnessctl in output: %s", output)
	}
	if !strings.Contains(output, "opentabletdriver") {
		t.Errorf("expected opentabletdriver in output: %s", output)
	}
}

func TestDiffPackagesMultiLineFalseCondition(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
packages = [
   """
   {{ if .laptop }}
   brightnessctl
   opentabletdriver
   {{ end }}
   """
]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["laptop"] = false

	output := captureStdout(func() {
		eng.diffPackages()
	})

	// Should not show any packages (condition false).
	if strings.Contains(output, "+ install") {
		t.Errorf("should not show packages when condition is false: %s", output)
	}
}

func TestDiffServicesMultiLine(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
services = [
    """
    {{ if .laptop }}
    brightnessctl
    opentabletdriver
    {{ end }}
    """
]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["laptop"] = true

	output := captureStdout(func() {
		eng.diffServices()
	})

	// Should show both services as needing enable.
	if !strings.Contains(output, "brightnessctl") {
		t.Errorf("expected brightnessctl in output: %s", output)
	}
	if !strings.Contains(output, "opentabletdriver") {
		t.Errorf("expected opentabletdriver in output: %s", output)
	}
}

func TestDiffServicesMultiLineFalseCondition(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := pkgManagerTOML(checkScript) + `
[mock]
services = [
    """
    {{ if .laptop }}
    brightnessctl
    opentabletdriver
    {{ end }}
    """
]
`
	cfg := loadTOML(t, sourceDir, tomlContent)
	eng := newEngine(t, sourceDir, cfg)
	eng.data["laptop"] = false

	output := captureStdout(func() {
		eng.diffServices()
	})

	// Should not show any services (condition false).
	if strings.Contains(output, "+ enable") {
		t.Errorf("should not show services when condition is false: %s", output)
	}
}
