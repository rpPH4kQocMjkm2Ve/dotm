package tmpl

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// FuncMap returns the custom template functions available in all templates.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"output":   fnOutput,
		"fromYaml": fnFromYaml,
		"joinPath": filepath.Join,
		"hasKey":   fnHasKey,
		"replace":  fnReplace,
	}
}

// output runs a command and returns its stdout, trimmed of trailing newlines.
//
// Usage in templates:
//
//	{{ output "sops" "-d" "secrets.enc.yaml" }}
//	{{ $s := output "sops" "-d" (joinPath .sourceDir "secrets.enc.yaml") | fromYaml }}
func fnOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s",
			name, strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimRight(stdout.String(), "\n"), nil
}

// fromYaml parses a YAML string into a map.
//
// Usage in templates:
//
//	{{ $s := output "sops" "-d" "file.yaml" | fromYaml }}
//	{{ index $s "key" }}
func fnFromYaml(s string) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal([]byte(s), &result); err != nil {
		return nil, fmt.Errorf("fromYaml: %w", err)
	}
	return result, nil
}

// hasKey checks whether a map contains a given key.
//
// Usage in templates:
//
//	{{ if hasKey $map "key" }}...{{ end }}
func fnHasKey(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

// replace replaces all occurrences of old with new in s.
// Argument order matches chezmoi: old, new, s.
//
// Usage in templates:
//
//	{{ replace "$HOME" .homeDir $path }}
func fnReplace(old, new, s string) string {
	return strings.ReplaceAll(s, old, new)
}
