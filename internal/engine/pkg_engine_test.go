package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
