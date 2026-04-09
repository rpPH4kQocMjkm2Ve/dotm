package engine

import (
	"os"
	"path/filepath"
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
	os.WriteFile(filepath.Join(sourceDir, "dotm.toml"), []byte(cfgContent), 0o644)

	// Create files/ directory.
	os.MkdirAll(filepath.Join(sourceDir, "files"), 0o755)

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
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write source %s: %v", rel, err)
	}
}

func writeDestFile(t *testing.T, destDir, rel, content string) {
	t.Helper()
	path := filepath.Join(destDir, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
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
	os.RemoveAll(filepath.Join(sourceDir, "files"))

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
