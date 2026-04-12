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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755) // not installed

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755) // installed

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755) // not enabled

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755) // enabled

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755)

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
	_, _, err = eng.renderName("{{ .Name }}")
	if err != nil {
		t.Fatalf("first renderName: %v", err)
	}

	_, _, err = eng.renderName("{{ .Name }}")
	if err != nil {
		t.Fatalf("second renderName: %v", err)
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
