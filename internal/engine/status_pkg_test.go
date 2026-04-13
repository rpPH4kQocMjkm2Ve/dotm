package engine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotm/internal/config"
	"dotm/internal/manifest"
	"dotm/internal/prompt"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("pipe: %v", err))
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		panic(fmt.Sprintf("w.Close: %v", err))
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		panic(fmt.Sprintf("io.Copy: %v", err))
	}
	return buf.String()
}

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("pipe: %v", err))
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	if err := w.Close(); err != nil {
		panic(fmt.Sprintf("w.Close: %v", err))
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		panic(fmt.Sprintf("io.Copy: %v", err))
	}
	return buf.String()
}

// managerTOML returns a minimal valid TOML config with a mock manager.
func managerTOML(checkScript string) string {
	return `
dest = "/tmp"
shell = "bash"

[managers.mock]
check = "` + checkScript + `"
install = "true"
remove = "true"
enable = "true"
disable = "true"
`
}

// pkgManagerTOML returns a minimal valid TOML config with a mock manager for packages.
// Deprecated: use managerTOML instead.
func pkgManagerTOML(checkScript string) string {
	return managerTOML(checkScript)
}

// svcManagerTOML returns a minimal valid TOML config with a mock manager for services.
// Deprecated: use managerTOML instead.
func svcManagerTOML(checkScript string) string {
	return managerTOML(checkScript)
}

func loadConfigWithPackages(t *testing.T, sourceDir, checkScript string, pkgs []string) *config.Config {
	t.Helper()
	tomlContent := pkgManagerTOML(checkScript) + "\n[mock]\npackages = ["
	for i, p := range pkgs {
		if i > 0 {
			tomlContent += ", "
		}
		tomlContent += `"` + p + `"`
	}
	tomlContent += "]\n"
	return loadTOML(t, sourceDir, tomlContent)
}

func loadConfigWithServices(t *testing.T, sourceDir, checkScript string, svcs []string) *config.Config {
	t.Helper()
	tomlContent := svcManagerTOML(checkScript) + "\n[mock]\nservices = ["
	for i, s := range svcs {
		if i > 0 {
			tomlContent += ", "
		}
		tomlContent += `"` + s + `"`
	}
	tomlContent += "]\n"
	return loadTOML(t, sourceDir, tomlContent)
}

func loadConfigNoPkgsOrSvcs(t *testing.T, sourceDir, checkScript string) *config.Config {
	t.Helper()
	return loadTOML(t, sourceDir, managerTOML(checkScript))
}

func loadTOML(t *testing.T, sourceDir, tomlContent string) *config.Config {
	t.Helper()
	tomlPath := filepath.Join(sourceDir, "dotm.toml")
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func newEngine(t *testing.T, sourceDir string, cfg *config.Config) *Engine {
	t.Helper()
	state := &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}
	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng
}

// ─── statusPackages ─────────────────────────────────────────────────────────

func TestStatusPackagesEmptyManagers(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash", Managers: map[string]config.ManagerConfig{}}
	eng := newEngine(t, sourceDir, cfg)

	report := &StatusReport{}
	if err := eng.statusPackages(report, &manifest.PkgManifest{}, false); err != nil {
		t.Fatalf("statusPackages: %v", err)
	}
}

func TestStatusPackagesInstalled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithPackages(t, sourceDir, checkScript+" {{.Name}}", []string{"git"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, &manifest.PkgManifest{}, true); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "OK") || !strings.Contains(output, "git") {
		t.Errorf("expected OK git in output, got: %s", output)
	}
}

func TestStatusPackagesMissing(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithPackages(t, sourceDir, checkScript+" {{.Name}}", []string{"vim"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, &manifest.PkgManifest{}, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "MISSING") {
		t.Errorf("expected MISSING in output, got: %s", output)
	}
}

func TestStatusPackagesTemplateEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithPackages(t, sourceDir, checkScript, []string{"{{ .pkg }}"})
	eng := newEngine(t, sourceDir, cfg)
	eng.data["pkg"] = ""

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, &manifest.PkgManifest{}, true); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "SKIP") {
		t.Errorf("expected SKIP in output, got: %s", output)
	}
}

func TestStatusPackagesTemplateError(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := loadConfigWithPackages(t, sourceDir, "true", []string{"{{ .missing }}"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, &manifest.PkgManifest{}, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "?") {
		t.Errorf("expected ? in output, got: %s", output)
	}
}

func TestStatusPackagesCheckError(t *testing.T) {
	sourceDir := t.TempDir()
	// Use a non-existent command — bash -c returns exit code 127, which the
	// engine treats as MISSING (not installed). Verify that behavior.
	cfg := loadConfigWithPackages(t, sourceDir, "/nonexistent/path/that/cannot/exec {{.Name}}", []string{"test"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, &manifest.PkgManifest{}, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	// Bash exit 127 is treated as "not installed" -> MISSING.
	if !strings.Contains(output, "MISSING") {
		t.Errorf("expected MISSING for non-existent command in output, got: %s", output)
	}
}

func TestStatusPackagesObsolete(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old", Manager: "mock"}},
	}

	cfg := loadConfigNoPkgsOrSvcs(t, sourceDir, checkScript)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, prevPkgs, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "OBSOLETE") {
		t.Errorf("expected OBSOLETE in output, got: %s", output)
	}
}

func TestStatusPackagesObsoleteNotInstalled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old", Manager: "mock"}},
	}

	cfg := loadConfigNoPkgsOrSvcs(t, sourceDir, checkScript)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, prevPkgs, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if strings.Contains(output, "OBSOLETE") {
		t.Errorf("should not print OBSOLETE, got: %s", output)
	}
}

func TestStatusPackagesObsoleteManagerMissing(t *testing.T) {
	sourceDir := t.TempDir()

	prevPkgs := &manifest.PkgManifest{
		Packages: []manifest.PackageEntry{{Name: "old", Manager: "missing"}},
	}

	cfg := loadConfigNoPkgsOrSvcs(t, sourceDir, "true")
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		report := &StatusReport{}
		if err := eng.statusPackages(report, prevPkgs, false); err != nil {
			t.Errorf("statusPackages: %v", err)
		}
	})

	if !strings.Contains(output, "WARN") {
		t.Errorf("expected WARN in stderr, got: %s", output)
	}
}

// ─── statusServices ─────────────────────────────────────────────────────────

func TestStatusServicesEmptyManagers(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := &config.Config{Dest: "/tmp", Shell: "bash", Managers: map[string]config.ManagerConfig{}}
	eng := newEngine(t, sourceDir, cfg)

	report := &StatusReport{}
	if err := eng.statusServices(report, &manifest.PkgManifest{}, false); err != nil {
		t.Fatalf("statusServices: %v", err)
	}
}

func TestStatusServicesEnabled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithServices(t, sourceDir, checkScript, []string{"sshd"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, &manifest.PkgManifest{}, true); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "ENABLED") || !strings.Contains(output, "sshd") {
		t.Errorf("expected ENABLED sshd in output, got: %s", output)
	}
}

func TestStatusServicesDisabled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithServices(t, sourceDir, checkScript, []string{"sshd"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, &manifest.PkgManifest{}, false); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "DISABLED") {
		t.Errorf("expected DISABLED in output, got: %s", output)
	}
}

func TestStatusServicesTemplateEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfigWithServices(t, sourceDir, checkScript, []string{"{{ .svc }}"})
	eng := newEngine(t, sourceDir, cfg)
	eng.data["svc"] = ""

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, &manifest.PkgManifest{}, true); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "SKIP") {
		t.Errorf("expected SKIP in output, got: %s", output)
	}
}

func TestStatusServicesTemplateError(t *testing.T) {
	sourceDir := t.TempDir()
	cfg := loadConfigWithServices(t, sourceDir, "true", []string{"{{ .missing }}"})
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, &manifest.PkgManifest{}, false); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "?") {
		t.Errorf("expected ? in output, got: %s", output)
	}
}

func TestStatusServicesObsoleteEnabled(t *testing.T) {
	sourceDir := t.TempDir()
	checkScript := filepath.Join(sourceDir, "check.sh")
	if err := os.WriteFile(checkScript, []byte("#!/bin/bash\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPkgs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "mock"}},
	}

	cfg := loadConfigNoPkgsOrSvcs(t, sourceDir, checkScript)
	eng := newEngine(t, sourceDir, cfg)

	output := captureStdout(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, prevPkgs, false); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "OBSOLETE") {
		t.Errorf("expected OBSOLETE in output, got: %s", output)
	}
}

func TestStatusServicesObsoleteManagerMissing(t *testing.T) {
	sourceDir := t.TempDir()

	prevPkgs := &manifest.PkgManifest{
		Services: []manifest.ServiceEntry{{Name: "old-svc", Manager: "missing"}},
	}

	cfg := loadConfigNoPkgsOrSvcs(t, sourceDir, "true")
	eng := newEngine(t, sourceDir, cfg)

	output := captureStderr(func() {
		report := &StatusReport{}
		if err := eng.statusServices(report, prevPkgs, false); err != nil {
			t.Errorf("statusServices: %v", err)
		}
	})

	if !strings.Contains(output, "WARN") {
		t.Errorf("expected WARN in stderr, got: %s", output)
	}
}
