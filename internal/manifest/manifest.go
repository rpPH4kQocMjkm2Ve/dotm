// Package manifest tracks installed packages and enabled services.
// It shares the same state file as prompt data — the state file
// contains data (prompts), script_hashes, manifest (files/dirs/symlinks),
// and now pkg_manifest (packages/services).
package manifest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// PackageEntry records an installed package and its manager.
type PackageEntry struct {
	Name    string `toml:"name"`
	Manager string `toml:"manager"`
}

// ServiceEntry records an enabled service and its manager.
type ServiceEntry struct {
	Name    string `toml:"name"`
	Manager string `toml:"manager"`
}

// PkgManifest tracks packages and services separately so we can
// distinguish obsolete entries from desired ones.
type PkgManifest struct {
	Packages []PackageEntry `toml:"packages"`
	Services []ServiceEntry `toml:"services"`
}

// testStateDir, if set, overrides the default state directory for testing.
var testStateDir string

// stateDir returns ~/.local/state/dotm, creating it if needed.
// If testStateDir is set (for testing), it is used instead.
func stateDir() (string, error) {
	if testStateDir != "" {
		return testStateDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "state", "dotm")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// stateFile returns the path to the state file for a given config directory.
// Uses the same SHA-256 hash scheme as prompt state.
func stateFile(configDir string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(configDir)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(abs))
	name := fmt.Sprintf("%x.toml", h[:8])
	return filepath.Join(dir, name), nil
}

// Load reads the package manifest from the state file.
// Returns an empty manifest if the file doesn't exist.
func Load(configDir string) (*PkgManifest, error) {
	path, err := stateFile(configDir)
	if err != nil {
		return nil, err
	}

	m := &PkgManifest{}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return m, nil
	}

	var raw map[string]any
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return m, fmt.Errorf("parse state %s: %w", path, err)
	}

	// Extract pkg_manifest section if present.
	if pmRaw, ok := raw["pkg_manifest"]; ok {
		pm, ok := pmRaw.(map[string]any)
		if !ok {
			return m, nil
		}

		m.Packages = decodePackageEntries(pm["packages"])
		m.Services = decodeServiceEntries(pm["services"])
	}

	return m, nil
}

// decodePackageEntries handles both []any and []map[string]any from TOML decode.
func decodePackageEntries(v any) []PackageEntry {
	var result []PackageEntry

	switch arr := v.(type) {
	case []map[string]any:
		for _, item := range arr {
			e := PackageEntry{}
			if n, ok := item["name"].(string); ok {
				e.Name = n
			}
			if mgr, ok := item["manager"].(string); ok {
				e.Manager = mgr
			}
			result = append(result, e)
		}
	case []any:
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				e := PackageEntry{}
				if n, ok := m["name"].(string); ok {
					e.Name = n
				}
				if mgr, ok := m["manager"].(string); ok {
					e.Manager = mgr
				}
				result = append(result, e)
			}
		}
	}

	return result
}

// decodeServiceEntries handles both []any and []map[string]any from TOML decode.
func decodeServiceEntries(v any) []ServiceEntry {
	var result []ServiceEntry

	switch arr := v.(type) {
	case []map[string]any:
		for _, item := range arr {
			e := ServiceEntry{}
			if n, ok := item["name"].(string); ok {
				e.Name = n
			}
			if mgr, ok := item["manager"].(string); ok {
				e.Manager = mgr
			}
			result = append(result, e)
		}
	case []any:
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				e := ServiceEntry{}
				if n, ok := m["name"].(string); ok {
					e.Name = n
				}
				if mgr, ok := m["manager"].(string); ok {
					e.Manager = mgr
				}
				result = append(result, e)
			}
		}
	}

	return result
}

// Save writes the package manifest into the state file.
// It merges with existing state data (prompts, script hashes, file manifest).
func Save(configDir string, m *PkgManifest) error {
	path, err := stateFile(configDir)
	if err != nil {
		return err
	}

	// Read existing state.
	var raw map[string]any
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &raw); err != nil {
			raw = make(map[string]any)
		}
	} else {
		raw = make(map[string]any)
	}

	// Build pkg_manifest section.
	pmRaw := make(map[string]any)
	if len(m.Services) > 0 {
		pmRaw["services"] = m.Services
	}
	if len(m.Packages) > 0 {
		pmRaw["packages"] = m.Packages
	}

	if len(pmRaw) > 0 {
		raw["pkg_manifest"] = pmRaw
	} else {
		delete(raw, "pkg_manifest")
	}

	// Write atomically.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := toml.NewEncoder(tmp).Encode(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}
