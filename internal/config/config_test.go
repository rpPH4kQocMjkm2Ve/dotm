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

// ─── parseGroup ──────────────────────────────────────────────────────────────

func TestParseGroupPackagesOnly(t *testing.T) {
	val := map[string]any{
		"packages": []any{"git", "zsh"},
	}
	g, err := parseGroup(val)
	if err != nil {
		t.Fatalf("parseGroup: %v", err)
	}
	if len(g.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(g.Packages))
	}
	if g.Packages[0].Name != "git" {
		t.Errorf("first package = %q, want git", g.Packages[0].Name)
	}
	if len(g.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(g.Services))
	}
}

func TestParseGroupServicesOnly(t *testing.T) {
	val := map[string]any{
		"services": []any{"firewalld", "sshd"},
	}
	g, err := parseGroup(val)
	if err != nil {
		t.Fatalf("parseGroup: %v", err)
	}
	if len(g.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(g.Services))
	}
	if g.Services[1].Name != "sshd" {
		t.Errorf("second service = %q, want sshd", g.Services[1].Name)
	}
}

func TestParseGroupMixed(t *testing.T) {
	val := map[string]any{
		"packages": []any{"git", map[string]any{"name": "zsh"}},
		"services": []any{map[string]any{"name": "firewalld"}},
	}
	g, err := parseGroup(val)
	if err != nil {
		t.Fatalf("parseGroup: %v", err)
	}
	if len(g.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(g.Packages))
	}
	if len(g.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(g.Services))
	}
}

func TestParseGroupInvalidType(t *testing.T) {
	_, err := parseGroup("not a map")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseGroupPackagesNotArray(t *testing.T) {
	val := map[string]any{
		"packages": "not an array",
	}
	_, err := parseGroup(val)
	if err == nil {
		t.Error("expected error for packages not array")
	}
}

func TestParseGroupServicesNotArray(t *testing.T) {
	val := map[string]any{
		"services": "not an array",
	}
	_, err := parseGroup(val)
	if err == nil {
		t.Error("expected error for services not array")
	}
}

func TestParseGroupUnknownKey(t *testing.T) {
	val := map[string]any{
		"packages":  []any{"git"},
		"unknownkey": "value",
	}
	_, err := parseGroup(val)
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestParseGroupPackageEntryNoName(t *testing.T) {
	val := map[string]any{
		"packages": []any{map[string]any{"foo": "bar"}},
	}
	_, err := parseGroup(val)
	if err == nil {
		t.Error("expected error for package without name")
	}
}

func TestParseGroupServiceEntryNoName(t *testing.T) {
	val := map[string]any{
		"services": []any{map[string]any{"foo": "bar"}},
	}
	_, err := parseGroup(val)
	if err == nil {
		t.Error("expected error for service without name")
	}
}

// ─── Packages / Services / HasPackages / HasServices ────────────────────────

func TestPackagesAndServices(t *testing.T) {
	cfg := &Config{
		Groups: map[string]GroupConfig{
			"pacman": {
				Packages: []NamedEntry{{Name: "git"}, {Name: "zsh"}},
			},
			"systemd": {
				Services: []NamedEntry{{Name: "firewalld"}},
			},
		},
	}
	cfg.resolvePackagesAndServices()

	pkgs := cfg.Packages()
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
	if pkgs[0].Manager != "pacman" {
		t.Errorf("first pkg manager = %q, want pacman", pkgs[0].Manager)
	}

	svcs := cfg.Services()
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}
	if svcs[0].Name != "firewalld" {
		t.Errorf("service name = %q, want firewalld", svcs[0].Name)
	}
}

func TestHasPackages(t *testing.T) {
	cfgWith := &Config{
		Groups: map[string]GroupConfig{
			"pacman": {Packages: []NamedEntry{{Name: "git"}}},
		},
	}
	cfgWith.resolvePackagesAndServices()
	if !cfgWith.HasPackages() {
		t.Error("expected HasPackages = true")
	}

	cfgWithout := &Config{
		Groups: map[string]GroupConfig{
			"systemd": {Services: []NamedEntry{{Name: "firewalld"}}},
		},
	}
	cfgWithout.resolvePackagesAndServices()
	if cfgWithout.HasPackages() {
		t.Error("expected HasPackages = false")
	}
}

func TestHasServices(t *testing.T) {
	cfgWith := &Config{
		Groups: map[string]GroupConfig{
			"systemd": {Services: []NamedEntry{{Name: "firewalld"}}},
		},
	}
	cfgWith.resolvePackagesAndServices()
	if !cfgWith.HasServices() {
		t.Error("expected HasServices = true")
	}

	cfgWithout := &Config{
		Groups: map[string]GroupConfig{
			"pacman": {Packages: []NamedEntry{{Name: "git"}}},
		},
	}
	cfgWithout.resolvePackagesAndServices()
	if cfgWithout.HasServices() {
		t.Error("expected HasServices = false")
	}
}

func TestPackagesServicesEmpty(t *testing.T) {
	cfg := &Config{
		Groups: map[string]GroupConfig{},
	}
	cfg.resolvePackagesAndServices()

	if len(cfg.Packages()) != 0 {
		t.Errorf("expected 0 packages, got %d", len(cfg.Packages()))
	}
	if len(cfg.Services()) != 0 {
		t.Errorf("expected 0 services, got %d", len(cfg.Services()))
	}
	if cfg.HasPackages() {
		t.Error("expected HasPackages = false")
	}
	if cfg.HasServices() {
		t.Error("expected HasServices = false")
	}
}

// ─── Load with groups and managers ──────────────────────────────────────────

func TestLoadWithManagersAndGroups(t *testing.T) {
	dir := t.TempDir()
	content := `
dest = "/home/user"

[managers.pacman]
check = "pacman -Q {{.Name}}"
install = "sudo pacman -S {{.Name}}"
remove = "sudo pacman -Rns {{.Name}}"

[pacman]
packages = ["git", "zsh"]
`
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Managers["pacman"]; !ok {
		t.Fatal("expected pacman manager")
	}
	if !cfg.HasPackages() {
		t.Error("expected HasPackages = true")
	}
	pkgs := cfg.Packages()
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestLoadUnknownManager(t *testing.T) {
	dir := t.TempDir()
	content := `
dest = "/home/user"

[unknown]
packages = ["git"]
`
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown manager")
	}
}

func TestLoadNoManagersNoDest(t *testing.T) {
	dir := t.TempDir()
	content := `
[unknown]
packages = ["git"]
`
	path := filepath.Join(dir, "dotm.toml")
	os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when no managers and no dest")
	}
}
