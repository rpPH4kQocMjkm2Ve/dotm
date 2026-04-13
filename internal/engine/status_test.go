package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotm/internal/config"
	"dotm/internal/prompt"
)

// helper to build a minimal engine for status tests.
func setupStatusTest(t *testing.T) (sourceDir string, destDir string, state *prompt.State) {
	t.Helper()

	sourceDir = t.TempDir()
	destDir = t.TempDir()

	// Create dotm.toml.
	cfgContent := `dest = "` + destDir + `"`
	if err := os.WriteFile(filepath.Join(sourceDir, "dotm.toml"), []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create files/ directory.
	if err := os.MkdirAll(filepath.Join(sourceDir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}

	state = &prompt.State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	return sourceDir, destDir, state
}

func buildEngine(t *testing.T, sourceDir, destDir string, state *prompt.State) *Engine {
	t.Helper()

	cfg, err := config.Load(filepath.Join(sourceDir, "dotm.toml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng
}

func writeSourceFile(t *testing.T, sourceDir, rel, content string) {
	t.Helper()
	path := filepath.Join(sourceDir, "files", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write source %s: %v", rel, err)
	}
}

func writeDestFile(t *testing.T, destDir, rel, content string) {
	t.Helper()
	path := filepath.Join(destDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write dest %s: %v", rel, err)
	}
}

func findEntry(report *StatusReport, rel string) *StatusEntry {
	for i := range report.Entries {
		if report.Entries[i].RelPath == rel {
			return &report.Entries[i]
		}
	}
	return nil
}

func TestStatusCleanFile(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	writeSourceFile(t, sourceDir, "test.conf", "hello")
	writeDestFile(t, destDir, "test.conf", "hello")

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "test.conf")
	if entry == nil {
		t.Fatal("expected entry for test.conf")
	}
	if entry.Status != StatusClean {
		t.Errorf("expected clean, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusModifiedFile(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	writeSourceFile(t, sourceDir, "test.conf", "source content")
	writeDestFile(t, destDir, "test.conf", "modified content")

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "test.conf")
	if entry == nil {
		t.Fatal("expected entry for test.conf")
	}
	if entry.Status != StatusModified {
		t.Errorf("expected modified, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusMissingFile(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	writeSourceFile(t, sourceDir, "test.conf", "hello")
	// Don't create in dest.

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "test.conf")
	if entry == nil {
		t.Fatal("expected entry for test.conf")
	}
	if entry.Status != StatusMissing {
		t.Errorf("expected missing, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusOrphanFile(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	// File NOT in source, but in manifest and dest.
	writeDestFile(t, destDir, "old.conf", "leftover")
	state.Manifest.Files = []string{"old.conf"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "old.conf")
	if entry == nil {
		t.Fatal("expected entry for old.conf")
	}
	if entry.Status != StatusOrphan {
		t.Errorf("expected orphan, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusOrphanNotInDest(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	// In manifest but gone from both source and dest — should not appear.
	state.Manifest.Files = []string{"gone.conf"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "gone.conf")
	if entry != nil {
		t.Error("expected no entry for gone.conf (not in dest)")
	}
}

func TestStatusTemplateFile(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Source has .tmpl file, dest has rendered result.
	writeSourceFile(t, sourceDir, "greet.txt.tmpl", "hello {{ .hostname }}")

	hostname, _ := os.Hostname()
	writeDestFile(t, destDir, "greet.txt", "hello "+hostname)

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	entry := findEntry(report, "greet.txt")
	if entry == nil {
		t.Fatal("expected entry for greet.txt")
	}
	if entry.Status != StatusClean {
		t.Errorf("expected clean, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusNoSourceDir(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	// Remove files/ directory.
	if err := os.RemoveAll(filepath.Join(sourceDir, "files")); err != nil {
		t.Fatal(err)
	}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(report.Entries))
	}
}

func TestStatusMixed(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	writeSourceFile(t, sourceDir, "clean.conf", "same")
	writeDestFile(t, destDir, "clean.conf", "same")

	writeSourceFile(t, sourceDir, "modified.conf", "new")
	writeDestFile(t, destDir, "modified.conf", "old")

	writeSourceFile(t, sourceDir, "missing.conf", "content")

	writeDestFile(t, destDir, "orphan.conf", "leftover")
	state.Manifest.Files = []string{"orphan.conf"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	clean, modified, missing, orphan := report.Counts()
	if clean != 1 {
		t.Errorf("expected 1 clean, got %d", clean)
	}
	if modified != 1 {
		t.Errorf("expected 1 modified, got %d", modified)
	}
	if missing != 1 {
		t.Errorf("expected 1 missing, got %d", missing)
	}
	if orphan != 1 {
		t.Errorf("expected 1 orphan, got %d", orphan)
	}

	if !report.HasProblems() {
		t.Error("expected HasProblems() to be true")
	}
}

func TestStatusSortedOutput(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Create files in non-alphabetical order.
	writeSourceFile(t, sourceDir, "zzz.conf", "z")
	writeDestFile(t, destDir, "zzz.conf", "z")

	writeSourceFile(t, sourceDir, "aaa.conf", "a")
	writeDestFile(t, destDir, "aaa.conf", "a")

	writeSourceFile(t, sourceDir, "mmm.conf", "m")
	writeDestFile(t, destDir, "mmm.conf", "m")

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(report.Entries))
	}
	if report.Entries[0].RelPath != "aaa.conf" {
		t.Errorf("expected first entry aaa.conf, got %s", report.Entries[0].RelPath)
	}
	if report.Entries[1].RelPath != "mmm.conf" {
		t.Errorf("expected second entry mmm.conf, got %s", report.Entries[1].RelPath)
	}
	if report.Entries[2].RelPath != "zzz.conf" {
		t.Errorf("expected third entry zzz.conf, got %s", report.Entries[2].RelPath)
	}
}

func TestStatusHasProblemsCleanOnly(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)
	writeSourceFile(t, sourceDir, "test.conf", "same")
	writeDestFile(t, destDir, "test.conf", "same")

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	if report.HasProblems() {
		t.Error("expected no problems for clean-only report")
	}
}

func TestStatusNestedFiles(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	writeSourceFile(t, sourceDir, ".config/hypr/hyprland.conf", "content")
	writeDestFile(t, destDir, ".config/hypr/hyprland.conf", "content")

	writeSourceFile(t, sourceDir, ".config/waybar/config.jsonc", "bar")
	// Missing from dest.

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeAll, false)
	if err != nil {
		t.Fatal(err)
	}

	hypr := findEntry(report, ".config/hypr/hyprland.conf")
	if hypr == nil || hypr.Status != StatusClean {
		t.Error("expected hyprland.conf clean")
	}

	waybar := findEntry(report, ".config/waybar/config.jsonc")
	if waybar == nil || waybar.Status != StatusMissing {
		t.Error("expected waybar config missing")
	}
}

// ─── FormatStatus ───────────────────────────────────────────────────────────

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		status FileStatus
		want   string
	}{
		{StatusClean, "clean"},
		{StatusModified, "modified"},
		{StatusMissing, "missing"},
		{StatusOrphan, "orphan"},
		{FileStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatStatus(tt.status)
			if got != tt.want {
				t.Errorf("FormatStatus(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ─── PrintReport ────────────────────────────────────────────────────────────

func TestPrintReportVerbose(t *testing.T) {
	report := &StatusReport{
		Entries: []StatusEntry{
			{".config/test.conf", StatusClean},
			{".config/other.conf", StatusModified},
		},
	}

	// Just verify it doesn't panic.
	PrintReport(report, true, ScopeFiles)
}

func TestPrintReportNonVerbose(t *testing.T) {
	report := &StatusReport{
		Entries: []StatusEntry{
			{".config/test.conf", StatusClean},
			{".config/other.conf", StatusModified},
		},
	}

	// Should only print non-clean entries.
	PrintReport(report, false, ScopeFiles)
}

func TestPrintReportEmpty(t *testing.T) {
	report := &StatusReport{}
	PrintReport(report, true, ScopeFiles)
}

func TestPrintReportWithPkgs(t *testing.T) {
	report := &StatusReport{
		Entries:        []StatusEntry{{".config/test.conf", StatusClean}},
		PkgHasProblems: true,
		PkgOrSvcPrinted: true,
	}
	PrintReport(report, false, ScopeAll)
}

// ─── Status with symlink in config ──────────────────────────────────────────

func TestStatusSymlinkMissing(t *testing.T) {
	sourceDir, _, state := setupStatusTest(t)

	// Add a symlink to config that doesn't exist in dest.
	cfg, err := config.Load(filepath.Join(sourceDir, "dotm.toml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Symlinks = map[string]string{"mylink": "/some/target"}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	entry := findEntry(report, "mylink")
	if entry == nil {
		t.Fatal("expected entry for symlink mylink")
	}
	if entry.Status != StatusMissing {
		t.Errorf("expected missing for symlink, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusSymlinkExists(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Create the symlink in dest.
	linkPath := filepath.Join(destDir, "mylink")
	if err := os.Symlink("/some/target", linkPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join(sourceDir, "dotm.toml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Symlinks = map[string]string{"mylink": "/some/target"}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	entry := findEntry(report, "mylink")
	if entry == nil {
		t.Fatal("expected entry for symlink mylink")
	}
	if entry.Status != StatusClean {
		t.Errorf("expected clean for symlink, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusSymlinkSkippedIfInSource(t *testing.T) {
	sourceDir, _, state := setupStatusTest(t)

	// Create a source file that matches symlink path.
	writeSourceFile(t, sourceDir, "mylink", "content")

	cfg, err := config.Load(filepath.Join(sourceDir, "dotm.toml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Symlinks = map[string]string{"mylink": "/some/target"}

	eng, err := New(cfg, state, sourceDir, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	// Should only have one entry (from source file, not symlink).
	count := 0
	for _, e := range report.Entries {
		if e.RelPath == "mylink" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for mylink, got %d", count)
	}
}

// ─── Status with orphan symlinks and directories ────────────────────────────

func TestStatusOrphanSymlink(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Create symlink in dest.
	linkPath := filepath.Join(destDir, "oldlink")
	if err := os.Symlink("/old/target", linkPath); err != nil {
		t.Fatal(err)
	}

	// Add to manifest.
	state.Manifest.Symlinks = []string{"oldlink"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	entry := findEntry(report, "oldlink")
	if entry == nil {
		t.Fatal("expected entry for orphan symlink oldlink")
	}
	if entry.Status != StatusOrphan {
		t.Errorf("expected orphan for symlink, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusOrphanDirectory(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Create directory in dest.
	dirPath := filepath.Join(destDir, "olddir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Add to manifest.
	state.Manifest.Directories = []string{"olddir"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	entry := findEntry(report, "olddir")
	if entry == nil {
		t.Fatal("expected entry for orphan directory olddir")
	}
	if entry.Status != StatusOrphan {
		t.Errorf("expected orphan for directory, got %s", FormatStatus(entry.Status))
	}
}

func TestStatusOrphanDirectoryNotInDest(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Don't create directory in dest.
	state.Manifest.Directories = []string{"gonedir"}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	entry := findEntry(report, "gonedir")
	if entry != nil {
		t.Error("expected no entry for orphan directory not in dest")
	}
}

// ─── Status with template rendering error ───────────────────────────────────

func TestStatusTemplateRenderError(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Source has template with missing key.
	writeSourceFile(t, sourceDir, "bad.txt.tmpl", "hello {{ .undefinedKey }}")
	// Create dest file so Status reaches the template render step
	// (without it, Status returns Missing before rendering).
	if err := os.WriteFile(filepath.Join(destDir, "bad.txt"), []byte("existing dest"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeFiles, false)
	// Status should return an error when template rendering fails.
	if err == nil {
		t.Error("expected error for template with missing key")
	} else if !strings.Contains(err.Error(), "undefinedKey") && !strings.Contains(err.Error(), "render") {
		t.Errorf("expected template error, got: %v", err)
	}
	_ = report
}

// ─── Status collectSourcePaths with ignore ───────────────────────────────────

func TestStatusCollectSourcePathsIgnoresDirectory(t *testing.T) {
	sourceDir, destDir, state := setupStatusTest(t)

	// Create files in a directory.
	writeSourceFile(t, sourceDir, ".git/config", "git config")
	writeSourceFile(t, sourceDir, ".git/HEAD", "ref: main")
	// Also create a non-ignored file to verify it appears.
	writeSourceFile(t, sourceDir, ".config/app.conf", "app config")

	// Create ignore.tmpl.
	if err := os.WriteFile(filepath.Join(sourceDir, "ignore.tmpl"), []byte(".git/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := buildEngine(t, sourceDir, destDir, state)
	report, err := eng.Status(ScopeFiles, false)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	// .git files should not appear in entries.
	for _, entry := range report.Entries {
		if strings.HasPrefix(entry.RelPath, ".git/") {
			t.Errorf("ignored file %s should not appear in status", entry.RelPath)
		}
	}

	// Non-ignored file should appear in entries.
	found := false
	for _, entry := range report.Entries {
		if entry.RelPath == ".config/app.conf" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .config/app.conf to appear in status entries")
	}
}

// ─── Counts edge cases ──────────────────────────────────────────────────────

func TestStatusCountsEmpty(t *testing.T) {
	report := &StatusReport{}
	clean, modified, missing, orphan := report.Counts()
	if clean != 0 || modified != 0 || missing != 0 || orphan != 0 {
		t.Errorf("expected all zeros, got clean=%d modified=%d missing=%d orphan=%d",
			clean, modified, missing, orphan)
	}
}

func TestStatusHasProblemsPkgSvc(t *testing.T) {
	report := &StatusReport{
		Entries:       []StatusEntry{{".config/test.conf", StatusClean}},
		PkgHasProblems: true,
	}
	if !report.HasProblems() {
		t.Error("expected HasProblems=true when PkgHasProblems")
	}

	report = &StatusReport{
		Entries:      []StatusEntry{{".config/test.conf", StatusClean}},
		SvcHasProblems: true,
	}
	if !report.HasProblems() {
		t.Error("expected HasProblems=true when SvcHasProblems")
	}
}
