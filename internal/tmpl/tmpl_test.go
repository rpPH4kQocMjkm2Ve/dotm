package tmpl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Render ──────────────────────────────────────────────────────────────────

func TestRenderPlainText(t *testing.T) {
	out, err := Render("hello world", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello world" {
		t.Errorf("got %q, want 'hello world'", out)
	}
}

func TestRenderVariable(t *testing.T) {
	data := map[string]any{"name": "alice"}
	out, err := Render("hello {{ .name }}", "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello alice" {
		t.Errorf("got %q", out)
	}
}

func TestRenderMissingKeyError(t *testing.T) {
	data := map[string]any{}
	_, err := Render("{{ .missing }}", "test", data)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRenderConditional(t *testing.T) {
	data := map[string]any{"gpu": true}
	out, err := Render("{{ if .gpu }}yes{{ else }}no{{ end }}", "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "yes" {
		t.Errorf("got %q, want 'yes'", out)
	}
}

func TestRenderConditionalFalse(t *testing.T) {
	data := map[string]any{"gpu": false}
	out, err := Render("{{ if .gpu }}yes{{ else }}no{{ end }}", "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "no" {
		t.Errorf("got %q, want 'no'", out)
	}
}

func TestRenderInvalidTemplate(t *testing.T) {
	_, err := Render("{{ .unclosed", "test", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// ─── RenderFile ──────────────────────────────────────────────────────────────

func TestRenderFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tmpl")
	os.WriteFile(path, []byte("host={{ .hostname }}"), 0o644)

	data := map[string]any{"hostname": "mybox"}
	out, err := RenderFile(path, data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "host=mybox" {
		t.Errorf("got %q", out)
	}
}

func TestRenderFileMissing(t *testing.T) {
	_, err := RenderFile("/nonexistent/file.tmpl", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ─── Template functions ──────────────────────────────────────────────────────

func TestFuncJoinPath(t *testing.T) {
	data := map[string]any{"dir": "/home/user"}
	out, err := Render(`{{ joinPath .dir "documents" "file.txt" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/home/user", "documents", "file.txt")
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestFuncHasKeyExists(t *testing.T) {
	data := map[string]any{
		"m": map[string]any{"key": "val"},
	}
	out, err := Render(`{{ if hasKey .m "key" }}yes{{ else }}no{{ end }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "yes" {
		t.Errorf("got %q, want 'yes'", out)
	}
}

func TestFuncHasKeyMissing(t *testing.T) {
	data := map[string]any{
		"m": map[string]any{"other": "val"},
	}
	out, err := Render(`{{ if hasKey .m "key" }}yes{{ else }}no{{ end }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "no" {
		t.Errorf("got %q, want 'no'", out)
	}
}

func TestFuncReplace(t *testing.T) {
	data := map[string]any{"home": "/home/alice"}
	out, err := Render(`{{ replace "$HOME" .home "/app/$HOME/config" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "/app/home/alice/config" {
		t.Errorf("got %q", out)
	}
}

func TestFuncFromYaml(t *testing.T) {
	data := map[string]any{
		"yamlStr": "key1: value1\nkey2: value2\n",
	}
	out, err := Render(`{{ $m := fromYaml .yamlStr }}{{ index $m "key1" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "value1" {
		t.Errorf("got %q, want 'value1'", out)
	}
}

func TestFuncFromYamlInvalid(t *testing.T) {
	data := map[string]any{
		"yamlStr": "not: valid: yaml: [[[",
	}
	_, err := Render(`{{ fromYaml .yamlStr }}`, "test", data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestFuncOutputEcho(t *testing.T) {
	// Use echo — available on all unix systems.
	out, err := Render(`{{ output "echo" "-n" "hello" }}`, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Errorf("got %q, want 'hello'", out)
	}
}

func TestFuncOutputTrimsNewline(t *testing.T) {
	out, err := Render(`{{ output "echo" "hello" }}`, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "\n") {
		t.Errorf("output should trim trailing newlines, got %q", out)
	}
}

func TestFuncOutputNonexistentCommand(t *testing.T) {
	_, err := Render(`{{ output "nonexistent_cmd_xyz_12345" }}`, "test", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
