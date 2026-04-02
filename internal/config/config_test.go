package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── expandHome ──────────────────────────────────────────────────────────────

func TestExpandHomeTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := expandHome("~")
	if got != home {
		t.Errorf("expandHome(~) = %q, want %q", got, home)
	}
}

func TestExpandHomeTildeSlash(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := expandHome("~/documents")
	want := filepath.Join(home, "documents")
	if got != want {
		t.Errorf("expandHome(~/documents) = %q, want %q", got, want)
	}
}

func TestExpandHomeAbsoluteUnchanged(t *testing.T) {
	got := expandHome("/etc/foo")
	if got != "/etc/foo" {
		t.Errorf("expandHome(/etc/foo) = %q, want /etc/foo", got)
	}
}

func TestExpandHomeRelativeUnchanged(t *testing.T) {
	got := expandHome("relative/path")
	if got != "relative/path" {
		t.Errorf("expandHome(relative/path) = %q, want relative/path", got)
	}
}

func TestExpandHomeEmpty(t *testing.T) {
	got := expandHome("")
	if got != "" {
		t.Errorf("expandHome('') = %q, want ''", got)
	}
}

// ─── validate ────────────────────────────────────────────────────────────────

func TestValidateDestRequired(t *testing.T) {
	cfg := &Config{Dest: ""}
	if err := cfg.validate(); err == nil {
		t.Error("expected error for empty dest")
	}
}

func TestValidateMinimalConfig(t *testing.T) {
	cfg := &Config{Dest: "/home/user"}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePromptUnknownType(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Prompts: map[string]PromptConfig{
			"foo": {Type: "integer", Question: "pick a number"},
		},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected error for unknown prompt type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("error %q should mention 'unknown type'", err)
	}
}

func TestValidatePromptBoolOk(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Prompts: map[string]PromptConfig{
			"flag": {Type: "bool", Question: "enable?"},
		},
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePromptStringOk(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Prompts: map[string]PromptConfig{
			"name": {Type: "string", Question: "your name?"},
		},
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePromptMissingQuestion(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Prompts: map[string]PromptConfig{
			"foo": {Type: "bool", Question: ""},
		},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected error for missing question")
	}
	if !strings.Contains(err.Error(), "question is required") {
		t.Errorf("error %q should mention 'question is required'", err)
	}
}

func TestValidateScriptPathRequired(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Scripts: []ScriptConfig{
			{Path: "", Trigger: "always"},
		},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected error for empty script path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("error %q should mention 'path is required'", err)
	}
}

func TestValidateScriptTriggerDefault(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Scripts: []ScriptConfig{
			{Path: "scripts/setup.sh", Trigger: ""},
		},
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg.Scripts[0].Trigger != "always" {
		t.Errorf("expected trigger 'always', got %q", cfg.Scripts[0].Trigger)
	}
}

func TestValidateScriptTriggerOnChange(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Scripts: []ScriptConfig{
			{Path: "scripts/setup.sh", Trigger: "on_change"},
		},
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateScriptTriggerUnknown(t *testing.T) {
	cfg := &Config{
		Dest: "/home/user",
		Scripts: []ScriptConfig{
			{Path: "scripts/setup.sh", Trigger: "never"},
		},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected error for unknown trigger")
	}
	if !strings.Contains(err.Error(), "unknown trigger") {
		t.Errorf("error %q should mention 'unknown trigger'", err)
	}
}

// ─── Load ────────────────────────────────────────────────────────────────────

func TestLoadMinimal(t *testing.T) {
	dir := t.TempDir()
	content := `dest = "/home/user"` + "\n"
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Dest != "/home/user" {
		t.Errorf("dest = %q, want /home/user", cfg.Dest)
	}
	if cfg.Shell != "bash" {
		t.Errorf("shell = %q, want bash", cfg.Shell)
	}
}

func TestLoadCustomShell(t *testing.T) {
	dir := t.TempDir()
	content := "dest = \"/home/user\"\nshell = \"zsh\"\n"
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Shell != "zsh" {
		t.Errorf("shell = %q, want zsh", cfg.Shell)
	}
}

func TestLoadExpandsHomeDest(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	dir := t.TempDir()
	content := `dest = "~"` + "\n"
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Dest != home {
		t.Errorf("dest = %q, want %q", cfg.Dest, home)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/dotm.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte("not valid [[[toml"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadMissingDest(t *testing.T) {
	dir := t.TempDir()
	content := "# no dest\n"
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing dest")
	}
	if !strings.Contains(err.Error(), "dest is required") {
		t.Errorf("error %q should mention 'dest is required'", err)
	}
}

func TestLoadWithPrompts(t *testing.T) {
	dir := t.TempDir()
	content := `
dest = "/home/user"

[prompts.use_gpu]
type = "bool"
question = "Enable GPU?"

[prompts.editor]
type = "string"
question = "Preferred editor?"
`
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Prompts) != 2 {
		t.Errorf("expected 2 prompts, got %d", len(cfg.Prompts))
	}
	if cfg.Prompts["use_gpu"].Type != "bool" {
		t.Errorf("use_gpu type = %q, want bool", cfg.Prompts["use_gpu"].Type)
	}
}

func TestLoadWithScripts(t *testing.T) {
	dir := t.TempDir()
	content := `
dest = "/home/user"

[[scripts]]
path = "scripts/setup.sh"
trigger = "on_change"
template = true
`
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
	}
	s := cfg.Scripts[0]
	if s.Path != "scripts/setup.sh" {
		t.Errorf("path = %q", s.Path)
	}
	if s.Trigger != "on_change" {
		t.Errorf("trigger = %q", s.Trigger)
	}
	if !s.Template {
		t.Error("expected template = true")
	}
}
