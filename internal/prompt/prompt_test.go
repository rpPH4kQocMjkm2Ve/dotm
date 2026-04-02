package prompt

import (
	"bytes"
	"strings"
	"testing"

	"dotm/internal/config"
)

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
