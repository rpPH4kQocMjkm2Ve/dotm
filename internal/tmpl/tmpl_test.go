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
	if err := os.WriteFile(path, []byte("host={{ .hostname }}"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	data := map[string]any{}
	// replace(old, new, s) — replaces all occurrences of old with new in s.
	out, err := Render(`{{ replace "world" "Go" "hello world" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello Go" {
		t.Errorf("got %q, want 'hello Go'", out)
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

// ─── default ─────────────────────────────────────────────────────────────────

func TestFuncDefaultWithValue(t *testing.T) {
	data := map[string]any{"key": "hello"}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Errorf("got %q, want 'hello'", out)
	}
}

func TestFuncDefaultWithNil(t *testing.T) {
	data := map[string]any{"key": nil}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "fallback" {
		t.Errorf("got %q, want 'fallback'", out)
	}
}

func TestFuncDefaultWithEmptyString(t *testing.T) {
	data := map[string]any{"key": ""}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "fallback" {
		t.Errorf("got %q, want 'fallback'", out)
	}
}

func TestFuncDefaultWithZeroInt(t *testing.T) {
	data := map[string]any{"key": 0}
	out, err := Render(`{{ .key | default 42 }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestFuncDefaultWithEmptySlice(t *testing.T) {
	data := map[string]any{"key": []any{}}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "fallback" {
		t.Errorf("got %q, want 'fallback'", out)
	}
}

func TestFuncDefaultWithEmptyMap(t *testing.T) {
	data := map[string]any{"key": map[string]any{}}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "fallback" {
		t.Errorf("got %q, want 'fallback'", out)
	}
}

func TestFuncDefaultWithFalse(t *testing.T) {
	data := map[string]any{"key": false}
	out, err := Render(`{{ .key | default true }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "true" {
		t.Errorf("got %q, want 'true'", out)
	}
}

func TestFuncDefaultWithNonEmptySlice(t *testing.T) {
	data := map[string]any{"key": []any{"item"}}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[item]" {
		t.Errorf("got %q, want '[item]'", out)
	}
}

func TestFuncDefaultChainedIndex(t *testing.T) {
	// Simulates the real-world pattern: index $s "app" "setting" | default "fallback"
	data := map[string]any{
		"s": map[string]any{
			"app": map[string]any{
				"setting": "value",
			},
		},
	}
	out, err := Render(`{{ index .s "app" "setting" | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "value" {
		t.Errorf("got %q, want 'value'", out)
	}
}

func TestFuncDefaultChainedIndexMissing(t *testing.T) {
	// First level exists but second key is missing — index returns nil, default provides fallback.
	data := map[string]any{
		"s": map[string]any{
			"app": map[string]any{},
		},
	}
	out, err := Render(`{{ index .s "app" "setting" | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "fallback" {
		t.Errorf("got %q, want 'fallback'", out)
	}
}

func TestFuncDefaultWithInt64(t *testing.T) {
	// TOML decodes integers as int64.
	data := map[string]any{"key": int64(42)}
	out, err := Render(`{{ .key | default 0 }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "42" {
		t.Errorf("got %q, want '42'", out)
	}
}

func TestFuncDefaultWithFloat64(t *testing.T) {
	data := map[string]any{"key": float64(3.14)}
	out, err := Render(`{{ .key | default 0.0 }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "3.14" {
		t.Errorf("got %q, want '3.14'", out)
	}
}

func TestFuncDefaultWithNonEmptyMap(t *testing.T) {
	data := map[string]any{"key": map[string]any{"a": 1}}
	out, err := Render(`{{ .key | default "fallback" }}`, "test", data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "map[a:1]" {
		t.Errorf("got %q, want 'map[a:1]'", out)
	}
}
