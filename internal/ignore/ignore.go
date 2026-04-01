package ignore

import (
	"os"
	"path/filepath"
	"strings"

	"dotm/internal/tmpl"
)

// Ignore holds compiled ignore patterns and tests paths against them.
type Ignore struct {
	patterns []string
}

// Load reads and renders ignore.tmpl from sourceDir, then parses
// the resulting lines into glob patterns.
// Returns an empty Ignore (matches nothing) if the file doesn't exist.
func Load(sourceDir string, data map[string]any) (*Ignore, error) {
	path := filepath.Join(sourceDir, "ignore.tmpl")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Ignore{}, nil
	}

	rendered, err := tmpl.RenderFile(path, data)
	if err != nil {
		return nil, err
	}

	var patterns []string
	for _, line := range strings.Split(string(rendered), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return &Ignore{patterns: patterns}, nil
}

// Match returns true if relPath should be ignored.
// relPath is relative to the files/ directory (e.g. ".config/hypr/hyprland.conf").
// Matching uses filepath.Match per segment for * patterns,
// and manual ** expansion for globstar patterns.
func (ig *Ignore) Match(relPath string) bool {
	for _, pattern := range ig.patterns {
		if matchGlob(pattern, relPath) {
			return true
		}
	}
	return false
}

// matchGlob tests a single pattern against a path.
// Supports ** (matches zero or more path segments), * and ? (single segment).
func matchGlob(pattern, path string) bool {
	// No ** — use filepath.Match directly.
	if !strings.Contains(pattern, "**") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := parts[1]

	// Clean up separators around **.
	prefix = strings.TrimSuffix(prefix, "/")
	suffix = strings.TrimPrefix(suffix, "/")

	if suffix == "" {
		// Pattern like "dir/**" — match dir itself and anything under it.
		if prefix == "" {
			return true // "**" matches everything
		}
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}

	if prefix == "" {
		// Pattern like "**/*.conf" — suffix can appear at any depth.
		segments := allSuffixes(path)
		for _, seg := range segments {
			matched, _ := filepath.Match(suffix, seg)
			if matched {
				return true
			}
		}
		return false
	}

	// Pattern like "dir/**/file" — prefix, then any depth, then suffix.
	if !strings.HasPrefix(path, prefix+"/") && path != prefix {
		return false
	}

	// Try matching suffix against every possible remainder.
	rest := strings.TrimPrefix(path, prefix+"/")
	// Zero intermediate segments: rest itself matches suffix.
	matched, _ := filepath.Match(suffix, rest)
	if matched {
		return true
	}
	// One or more intermediate segments.
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			candidate := rest[i+1:]
			matched, _ := filepath.Match(suffix, candidate)
			if matched {
				return true
			}
		}
	}
	return false
}

// allSuffixes returns the path and all sub-paths:
// "a/b/c.conf" → ["a/b/c.conf", "b/c.conf", "c.conf"]
func allSuffixes(path string) []string {
	var result []string
	result = append(result, path)
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			result = append(result, path[i+1:])
		}
	}
	return result
}
