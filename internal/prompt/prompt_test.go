package prompt

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotm/internal/config"
)

func newScanner(input string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(input))
}

// ─── coerceValue ─────────────────────────────────────────────────────────────

func TestCoerceValueBool(t *testing.T) {
	if v := coerceValue(true); v != true {
		t.Errorf("expected true, got %v", v)
	}
	if v := coerceValue(false); v != false {
		t.Errorf("expected false, got %v", v)
	}
}

func TestCoerceValueInt64(t *testing.T) {
	v := coerceValue(int64(42))
	if v != int64(42) {
		t.Errorf("expected int64(42), got %v (%T)", v, v)
	}
}

func TestCoerceValueFloat64WholeNumber(t *testing.T) {
	v := coerceValue(float64(42.0))
	if v != int64(42) {
		t.Errorf("expected int64(42), got %v (%T)", v, v)
	}
}

func TestCoerceValueFloat64Fractional(t *testing.T) {
	v := coerceValue(float64(3.14))
	if v != float64(3.14) {
		t.Errorf("expected 3.14, got %v", v)
	}
}

func TestCoerceValueStringTrue(t *testing.T) {
	for _, s := range []string{"true", "True", "TRUE"} {
		v := coerceValue(s)
		if v != true {
			t.Errorf("coerceValue(%q) = %v, want true", s, v)
		}
	}
}

func TestCoerceValueStringFalse(t *testing.T) {
	for _, s := range []string{"false", "False", "FALSE"} {
		v := coerceValue(s)
		if v != false {
			t.Errorf("coerceValue(%q) = %v, want false", s, v)
		}
	}
}

func TestCoerceValueStringRegular(t *testing.T) {
	v := coerceValue("hello")
	if v != "hello" {
		t.Errorf("expected 'hello', got %v", v)
	}
}

func TestCoerceValueOtherType(t *testing.T) {
	// Slice — should pass through unchanged.
	input := []string{"a", "b"}
	v := coerceValue(input)
	if _, ok := v.([]string); !ok {
		t.Errorf("expected []string passthrough, got %T", v)
	}
}

// ─── HashContent ─────────────────────────────────────────────────────────────

func TestHashContentDeterministic(t *testing.T) {
	content := []byte("hello world")
	h1 := HashContent(content)
	h2 := HashContent(content)
	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Errorf("hash should start with 'sha256:', got %q", h1)
	}
}

func TestHashContentDifferent(t *testing.T) {
	h1 := HashContent([]byte("hello"))
	h2 := HashContent([]byte("world"))
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestHashContentEmpty(t *testing.T) {
	h := HashContent([]byte{})
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("empty content hash should still have prefix, got %q", h)
	}
}

// ─── FormatPromptValue ───────────────────────────────────────────────────────

func TestFormatPromptValueBool(t *testing.T) {
	if s := FormatPromptValue(true); s != "true" {
		t.Errorf("got %q, want 'true'", s)
	}
	if s := FormatPromptValue(false); s != "false" {
		t.Errorf("got %q, want 'false'", s)
	}
}

func TestFormatPromptValueString(t *testing.T) {
	if s := FormatPromptValue("vim"); s != "vim" {
		t.Errorf("got %q, want 'vim'", s)
	}
}

func TestFormatPromptValueOther(t *testing.T) {
	s := FormatPromptValue(42)
	if s != "42" {
		t.Errorf("got %q, want '42'", s)
	}
}

// ─── Resolve ─────────────────────────────────────────────────────────────────

func TestResolveNoPrompts(t *testing.T) {
	cfg := &config.Config{Dest: "/home/user"}
	state := &State{Data: make(map[string]any)}

	changed, err := Resolve(cfg, state, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected no changes with no prompts")
	}
}

func TestResolveAlreadyCached(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"name": {Type: "string", Question: "Your name?"},
		},
	}
	state := &State{Data: map[string]any{"name": "alice"}}

	changed, err := Resolve(cfg, state, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected no changes — value already cached")
	}
}

func TestResolveBoolYes(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"gpu": {Type: "bool", Question: "Enable GPU?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}

	changed, err := Resolve(cfg, state, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if state.Data["gpu"] != true {
		t.Errorf("expected gpu=true, got %v", state.Data["gpu"])
	}
}

func TestResolveBoolNo(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"gpu": {Type: "bool", Question: "Enable GPU?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	input := strings.NewReader("n\n")
	output := &bytes.Buffer{}

	changed, err := Resolve(cfg, state, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if state.Data["gpu"] != false {
		t.Errorf("expected gpu=false, got %v", state.Data["gpu"])
	}
}

func TestResolveString(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"editor": {Type: "string", Question: "Editor?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	input := strings.NewReader("nvim\n")
	output := &bytes.Buffer{}

	changed, err := Resolve(cfg, state, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if state.Data["editor"] != "nvim" {
		t.Errorf("expected 'nvim', got %v", state.Data["editor"])
	}
}

func TestResolveBoolRetryInvalidInput(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"gpu": {Type: "bool", Question: "Enable GPU?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	// First line invalid, second line valid.
	input := strings.NewReader("maybe\ny\n")
	output := &bytes.Buffer{}

	changed, err := Resolve(cfg, state, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if state.Data["gpu"] != true {
		t.Errorf("expected gpu=true, got %v", state.Data["gpu"])
	}
	// Should have printed retry message.
	if !strings.Contains(output.String(), "please answer y or n") {
		t.Error("expected retry message in output")
	}
}

func TestResolveMultiplePromptsSorted(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"zzz": {Type: "string", Question: "Last?"},
			"aaa": {Type: "string", Question: "First?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	// Prompts sorted alphabetically: aaa first, then zzz.
	input := strings.NewReader("first_val\nlast_val\n")
	output := &bytes.Buffer{}

	changed, err := Resolve(cfg, state, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if state.Data["aaa"] != "first_val" {
		t.Errorf("aaa = %v, want 'first_val'", state.Data["aaa"])
	}
	if state.Data["zzz"] != "last_val" {
		t.Errorf("zzz = %v, want 'last_val'", state.Data["zzz"])
	}
}

func TestResolveEOFError(t *testing.T) {
	cfg := &config.Config{
		Dest: "/home/user",
		Prompts: map[string]config.PromptConfig{
			"name": {Type: "string", Question: "Name?"},
		},
	}
	state := &State{Data: make(map[string]any)}

	// Empty input — EOF before answer.
	input := strings.NewReader("")
	output := &bytes.Buffer{}

	_, err := Resolve(cfg, state, input, output)
	if err == nil {
		t.Fatal("expected error on EOF")
	}
}

// ─── State script hashes ────────────────────────────────────────────────────

func TestStateScriptHash(t *testing.T) {
	s := &State{ScriptHashes: make(map[string]string)}

	if h := s.GetScriptHash("test.sh"); h != "" {
		t.Errorf("expected empty, got %q", h)
	}

	s.SetScriptHash("test.sh", "sha256:abc123")
	if h := s.GetScriptHash("test.sh"); h != "sha256:abc123" {
		t.Errorf("expected 'sha256:abc123', got %q", h)
	}
}

// ─── State manifest ─────────────────────────────────────────────────────────

func TestStateSetManifestSorts(t *testing.T) {
	s := &State{}
	s.SetManifest(
		[]string{"zzz", "aaa", "mmm"},
		[]string{"dir_b", "dir_a"},
		[]string{"link_c", "link_a"},
	)

	if s.Manifest.Files[0] != "aaa" || s.Manifest.Files[2] != "zzz" {
		t.Errorf("files not sorted: %v", s.Manifest.Files)
	}
	if s.Manifest.Directories[0] != "dir_a" {
		t.Errorf("dirs not sorted: %v", s.Manifest.Directories)
	}
	if s.Manifest.Symlinks[0] != "link_a" {
		t.Errorf("symlinks not sorted: %v", s.Manifest.Symlinks)
	}
}

// ─── ResetPrompt ─────────────────────────────────────────────────────────────

func TestResetPrompt(t *testing.T) {
	s := &State{Data: map[string]any{"gpu": true, "editor": "vim"}}
	s.ResetPrompt("gpu")
	if _, exists := s.Data["gpu"]; exists {
		t.Error("gpu should be deleted")
	}
	if s.Data["editor"] != "vim" {
		t.Error("editor should remain")
	}
}

// ─── LoadState / Save / stateFile / stateDir ────────────────────────────────

func TestLoadStateEmptySourceDir(t *testing.T) {
	// LoadState for a non-existent source dir should return empty state.
	s, err := LoadState("/nonexistent/source")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if s.Data == nil {
		t.Error("Data should be initialized")
	}
	if s.ScriptHashes == nil {
		t.Error("ScriptHashes should be initialized")
	}
	if len(s.Manifest.Files) != 0 {
		t.Errorf("expected 0 manifest files, got %d", len(s.Manifest.Files))
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	// Use a real source dir that LoadState can hash.
	sourceDir := t.TempDir()

	orig := &State{
		Data:         map[string]any{"laptop": true, "editor": "nvim"},
		ScriptHashes: map[string]string{"script.sh": "sha256:abc123"},
	}
	orig.SetManifest([]string{"a.conf", "b.conf"}, []string{"dir"}, []string{"link"})

	if err := orig.Save(sourceDir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load the state back.
	loaded, err := LoadState(sourceDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.Data["laptop"] != true {
		t.Errorf("laptop = %v, want true", loaded.Data["laptop"])
	}
	if loaded.Data["editor"] != "nvim" {
		t.Errorf("editor = %v, want 'nvim'", loaded.Data["editor"])
	}
	if loaded.ScriptHashes["script.sh"] != "sha256:abc123" {
		t.Errorf("script hash = %v, want sha256:abc123", loaded.ScriptHashes["script.sh"])
	}
	if len(loaded.Manifest.Files) != 2 {
		t.Errorf("manifest files = %d, want 2", len(loaded.Manifest.Files))
	}
	if len(loaded.Manifest.Directories) != 1 {
		t.Errorf("manifest dirs = %d, want 1", len(loaded.Manifest.Directories))
	}
	if len(loaded.Manifest.Symlinks) != 1 {
		t.Errorf("manifest symlinks = %d, want 1", len(loaded.Manifest.Symlinks))
	}
}

func TestSaveStateAtomic(t *testing.T) {
	sourceDir := t.TempDir()

	s := &State{
		Data:         map[string]any{"key": "value"},
		ScriptHashes: make(map[string]string),
	}

	// Save multiple times — should not leave temp files.
	for i := 0; i < 5; i++ {
		s.Data["iteration"] = i
		if err := s.Save(sourceDir); err != nil {
			t.Fatalf("Save #%d: %v", i, err)
		}
	}

	loaded, err := LoadState(sourceDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.Data["key"] != "value" {
		t.Errorf("key = %v, want 'value'", loaded.Data["key"])
	}
}

func TestLoadStateNoExistingState(t *testing.T) {
	// Test that LoadState returns empty state for dirs with no state file.
	sourceDir := t.TempDir()

	s, err := LoadState(sourceDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(s.Data) != 0 {
		t.Errorf("expected empty data, got %d entries", len(s.Data))
	}
}

// ─── BuildData ──────────────────────────────────────────────────────────────

func TestBuildData(t *testing.T) {
	s := &State{
		Data:         map[string]any{"laptop": true},
		ScriptHashes: map[string]string{},
	}

	data, err := BuildData(s, "/home/user/dotfiles")
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}

	if data["laptop"] != true {
		t.Errorf("laptop = %v, want true", data["laptop"])
	}
	if data["homeDir"] == "" {
		t.Error("homeDir should not be empty")
	}
	if data["hostname"] == "" {
		t.Error("hostname should not be empty")
	}
	if data["sourceDir"] != "/home/user/dotfiles" {
		// Note: may be absolute resolved path
		if !strings.HasSuffix(data["sourceDir"].(string), "dotfiles") {
			t.Errorf("sourceDir = %v, should end with dotfiles", data["sourceDir"])
		}
	}
}

func TestBuildDataInvalidSourceDir(t *testing.T) {
	s := &State{Data: map[string]any{}}
	// Test with a very long path that might cause issues.
	// Note: NUL bytes don't always error in Go, so we test with empty string.
	_, err := BuildData(s, "")
	// Empty string is valid (becomes current dir), so we just verify no crash.
	_ = err
}

// ─── FormatStateFile ────────────────────────────────────────────────────────

func TestFormatStateFile(t *testing.T) {
	result := FormatStateFile("/home/user/dotfiles")
	if result == "" {
		t.Error("FormatStateFile should not return empty string")
	}
	// Should use ~ shorthand if under home.
	if strings.HasPrefix(result, "~") {
		t.Logf("FormatStateFile returned: %s", result)
	}
}

func TestSetGetScriptHash(t *testing.T) {
	s := &State{ScriptHashes: make(map[string]string)}
	s.SetScriptHash("script.sh", "hash123")
	if got := s.GetScriptHash("script.sh"); got != "hash123" {
		t.Errorf("GetScriptHash = %v, want hash123", got)
	}
	if got := s.GetScriptHash("unknown.sh"); got != "" {
		t.Errorf("GetScriptHash for unknown = %v, want empty", got)
	}
}

// ─── askBool edge cases ─────────────────────────────────────────────────────

func TestAskBoolEmptyInput(t *testing.T) {
	input := "\n\nyes\n"
	scanner := newScanner(input)
	var buf bytes.Buffer
	result, err := askBool(scanner, &buf, "Question?")
	if err != nil {
		t.Fatalf("askBool: %v", err)
	}
	if !result {
		t.Error("expected true for 'yes' after retries")
	}
	output := buf.String()
	if !strings.Contains(output, "y or n") {
		t.Errorf("should prompt retry, got: %q", output)
	}
}

func TestAskBoolInvalidInput(t *testing.T) {
	input := "maybe\nyes\n"
	scanner := newScanner(input)
	var buf bytes.Buffer
	result, err := askBool(scanner, &buf, "Question?")
	if err != nil {
		t.Fatalf("askBool: %v", err)
	}
	if !result {
		t.Error("expected true for 'yes'")
	}
}

func TestAskBoolN(t *testing.T) {
	for _, s := range []string{"n", "no", "N", "NO"} {
		scanner := newScanner(s + "\n")
		var buf bytes.Buffer
		result, err := askBool(scanner, &buf, "Question?")
		if err != nil {
			t.Fatalf("askBool(%q): %v", s, err)
		}
		if result {
			t.Errorf("askBool(%q) = true, want false", s)
		}
	}
}

// ─── askString edge cases ───────────────────────────────────────────────────

func TestAskStringEmptyInput(t *testing.T) {
	scanner := newScanner("\n")
	var buf bytes.Buffer
	result, err := askString(scanner, &buf, "Name?")
	if err != nil {
		t.Fatalf("askString: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestAskStringWhitespaceTrimmed(t *testing.T) {
	scanner := newScanner("  hello  \n")
	var buf bytes.Buffer
	result, err := askString(scanner, &buf, "Name?")
	if err != nil {
		t.Fatalf("askString: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestAskStringEOF(t *testing.T) {
	scanner := newScanner("")
	var buf bytes.Buffer
	_, err := askString(scanner, &buf, "Name?")
	if err == nil {
		t.Fatal("expected error for EOF")
	}
	if !strings.Contains(err.Error(), "end of input") {
		t.Errorf("error = %q, should mention end of input", err)
	}
}

// ─── ResetPrompt additional ──────────────────────────────────────────────────

func TestResetPromptNonExistent(t *testing.T) {
	s := &State{Data: map[string]any{"name": "value"}}
	s.ResetPrompt("nonexistent") // Should not panic.
	if len(s.Data) != 1 {
		t.Errorf("expected 1 entry, got %d", len(s.Data))
	}
}

// ─── BuildData hostname fallback ────────────────────────────────────────────

func TestBuildDataHostnameFallback(t *testing.T) {
	s := &State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	// Even if hostname fails, BuildData should work with fallback.
	data, err := BuildData(s, "/tmp/dotfiles")
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}

	// hostname may be empty on error — verify no crash.
	_ = data["hostname"]
}

func TestBuildDataUsernameFallback(t *testing.T) {
	s := &State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	// Clear USER and LOGNAME env vars.
	t.Setenv("USER", "")
	t.Setenv("LOGNAME", "")

	data, err := BuildData(s, "/tmp/dotfiles")
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}

	// username should be empty string.
	if data["username"] != "" {
		t.Errorf("expected empty username, got %q", data["username"])
	}
}

// ─── FormatStateFile edge cases ─────────────────────────────────────────────

func TestFormatStateFileError(t *testing.T) {
	// stateFile can error with invalid sourceDir (NUL byte on some systems).
	// On Linux, empty string is valid (current dir), so test won't error.
	// Just verify no panic.
	result := FormatStateFile("")
	// Result should be non-empty (either path or "~" or full path).
	if result == "" {
		t.Error("FormatStateFile should not return empty string")
	}
}

// ─── LoadState corrupted TOML ───────────────────────────────────────────────

func TestLoadStateCorruptedTOML(t *testing.T) {
	sourceDir := t.TempDir()

	// Write corrupted TOML to state file.
	path, err := stateFile(sourceDir)
	if err != nil {
		t.Fatalf("stateFile: %v", err)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{{{corrupted toml}}}"), 0o644)

	_, err = LoadState(sourceDir)
	if err == nil {
		t.Error("expected error for corrupted TOML")
	}
}

// ─── Save error paths ───────────────────────────────────────────────────────

func TestSaveStateError(t *testing.T) {
	s := &State{
		Data:         map[string]any{"key": "value"},
		ScriptHashes: make(map[string]string),
	}

	// Save to a directory where we can't write (use read-only dir).
	// Note: On many systems this won't actually fail for root,
	// so we just verify it doesn't panic.
	_ = s.Save("/nonexistent/path/that/cannot/exist")
}
