package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── matchGlob ───────────────────────────────────────────────────────────────

func TestMatchGlobExact(t *testing.T) {
	if !matchGlob(".config/hypr/hyprland.conf", ".config/hypr/hyprland.conf") {
		t.Error("exact match should succeed")
	}
}

func TestMatchGlobExactNoMatch(t *testing.T) {
	if matchGlob(".config/hypr/hyprland.conf", ".config/hypr/other.conf") {
		t.Error("different file should not match")
	}
}

func TestMatchGlobStar(t *testing.T) {
	if !matchGlob("*.conf", "test.conf") {
		t.Error("*.conf should match test.conf")
	}
}

func TestMatchGlobStarNoNested(t *testing.T) {
	if matchGlob("*.conf", "dir/test.conf") {
		t.Error("*.conf should not match nested path")
	}
}

func TestMatchGlobDoubleStarSuffix(t *testing.T) {
	if !matchGlob(".config/**", ".config/hypr/hyprland.conf") {
		t.Error("** should match nested path")
	}
}

func TestMatchGlobDoubleStarDirect(t *testing.T) {
	if !matchGlob(".config/**", ".config/test.conf") {
		t.Error("** should match direct child")
	}
}

func TestMatchGlobDoubleStarPrefix(t *testing.T) {
	if !matchGlob("**/*.bak", "deep/nested/file.bak") {
		t.Error("**/*.bak should match nested .bak file")
	}
}

func TestMatchGlobDoubleStarPrefixDirect(t *testing.T) {
	if !matchGlob("**/*.bak", "file.bak") {
		t.Error("**/*.bak should match direct .bak file")
	}
}

func TestMatchGlobDoubleStarMiddle(t *testing.T) {
	if !matchGlob(".config/**/conf.d", ".config/fontconfig/conf.d") {
		t.Error("should match with one intermediate segment")
	}
}

func TestMatchGlobDoubleStarMiddleZero(t *testing.T) {
	if !matchGlob(".config/**/conf.d", ".config/conf.d") {
		t.Error("should match with zero intermediate segments")
	}
}

func TestMatchGlobDoubleStarMiddleDeep(t *testing.T) {
	if !matchGlob(".config/**/conf.d", ".config/a/b/c/conf.d") {
		t.Error("should match with multiple intermediate segments")
	}
}

func TestMatchGlobNoMatchOutside(t *testing.T) {
	if matchGlob(".config/**", ".local/bin/script") {
		t.Error("should not match outside prefix")
	}
}

func TestMatchGlobQuestionMark(t *testing.T) {
	if !matchGlob("?.txt", "a.txt") {
		t.Error("? should match single char")
	}
	if matchGlob("?.txt", "ab.txt") {
		t.Error("? should not match two chars")
	}
}

func TestMatchGlobEverything(t *testing.T) {
	if !matchGlob("**", "anything/at/all") {
		t.Error("** alone should match everything")
	}
}

// ─── Ignore.Match ────────────────────────────────────────────────────────────

func TestIgnoreMatchEmpty(t *testing.T) {
	ig := &Ignore{}
	if ig.Match(".config/test") {
		t.Error("empty ignore should match nothing")
	}
}

func TestIgnoreMatchSinglePattern(t *testing.T) {
	ig := &Ignore{patterns: []string{"*.bak"}}
	if !ig.Match("test.bak") {
		t.Error("should match *.bak")
	}
	if ig.Match("test.conf") {
		t.Error("should not match .conf")
	}
}

func TestIgnoreMatchMultiplePatterns(t *testing.T) {
	ig := &Ignore{patterns: []string{"*.bak", ".git/**"}}
	if !ig.Match("test.bak") {
		t.Error("should match *.bak")
	}
	if !ig.Match(".git/config") {
		t.Error("should match .git/**")
	}
	if ig.Match("README.md") {
		t.Error("should not match README.md")
	}
}

// ─── Load ────────────────────────────────────────────────────────────────────

func TestLoadNoFile(t *testing.T) {
	dir := t.TempDir()
	ig, err := Load(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ig.Match("anything") {
		t.Error("should match nothing when no ignore file exists")
	}
}

func TestLoadWithPatterns(t *testing.T) {
	dir := t.TempDir()
	content := "*.bak\n# comment\n\n.git/**\n"
	os.WriteFile(filepath.Join(dir, "ignore.tmpl"), []byte(content), 0o644)

	data := map[string]any{
		"homeDir":   "/home/test",
		"hostname":  "testhost",
		"username":  "testuser",
		"sourceDir": dir,
	}

	ig, err := Load(dir, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ig.Match("test.bak") {
		t.Error("should match *.bak")
	}
	if !ig.Match(".git/config") {
		t.Error("should match .git/**")
	}
	if ig.Match("README.md") {
		t.Error("should not match README.md")
	}
}

func TestLoadWithTemplate(t *testing.T) {
	dir := t.TempDir()
	// Template that conditionally adds a pattern.
	content := `{{ if .skipGames }}games/**{{ end }}
*.tmp
`
	os.WriteFile(filepath.Join(dir, "ignore.tmpl"), []byte(content), 0o644)

	data := map[string]any{
		"skipGames": true,
		"homeDir":   "/home/test",
		"hostname":  "testhost",
		"username":  "testuser",
		"sourceDir": dir,
	}

	ig, err := Load(dir, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ig.Match("games/doom/config") {
		t.Error("should match games/** when skipGames=true")
	}
	if !ig.Match("notes.tmp") {
		t.Error("should match *.tmp")
	}
}

func TestLoadWithTemplateFalse(t *testing.T) {
	dir := t.TempDir()
	content := `{{ if .skipGames }}games/**{{ end }}
*.tmp
`
	os.WriteFile(filepath.Join(dir, "ignore.tmpl"), []byte(content), 0o644)

	data := map[string]any{
		"skipGames": false,
		"homeDir":   "/home/test",
		"hostname":  "testhost",
		"username":  "testuser",
		"sourceDir": dir,
	}

	ig, err := Load(dir, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ig.Match("games/doom/config") {
		t.Error("should NOT match games/** when skipGames=false")
	}
	if !ig.Match("notes.tmp") {
		t.Error("should still match *.tmp")
	}
}

func TestLoadCommentsAndBlanksSkipped(t *testing.T) {
	dir := t.TempDir()
	content := "# full line comment\n\n   # indented comment\n   \nactual.pattern\n"
	os.WriteFile(filepath.Join(dir, "ignore.tmpl"), []byte(content), 0o644)

	data := map[string]any{
		"homeDir":   "/home/test",
		"hostname":  "testhost",
		"username":  "testuser",
		"sourceDir": dir,
	}

	ig, err := Load(dir, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ig.patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d: %v", len(ig.patterns), ig.patterns)
	}
	if ig.patterns[0] != "actual.pattern" {
		t.Errorf("pattern = %q, want 'actual.pattern'", ig.patterns[0])
	}
}
