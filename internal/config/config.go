package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the dotm.toml configuration file.
type Config struct {
	Dest     string                  `toml:"dest"`
	Shell    string                  `toml:"shell"`
	Prompts  map[string]PromptConfig `toml:"prompts"`
	Symlinks map[string]string       `toml:"symlinks"`
	Scripts  []ScriptConfig          `toml:"scripts"`

	// Package/service management (from pkgm).
	Managers map[string]ManagerConfig `toml:"managers"`
	Groups   map[string]GroupConfig

	// Resolved packages and services, computed during Load.
	resolvedPkgs []PackageEntry
	resolvedSvcs []ServiceEntry
}

// PromptConfig defines an interactive prompt.
type PromptConfig struct {
	Type     string `toml:"type"`     // "bool" or "string"
	Question string `toml:"question"` // displayed to user
}

// ScriptConfig defines a lifecycle script.
type ScriptConfig struct {
	Path     string `toml:"path"`
	Template bool   `toml:"template"`
	Trigger  string `toml:"trigger"` // "always" or "on_change"
}

// ManagerConfig defines a package/service manager command template.
type ManagerConfig struct {
	Check   string `toml:"check"`
	Install string `toml:"install"`
	Remove  string `toml:"remove"`
	Enable  string `toml:"enable"`
	Disable string `toml:"disable"`
}

// GroupConfig holds packages and services for a manager.
type GroupConfig struct {
	Packages []NamedEntry
	Services []NamedEntry
}

// NamedEntry is a package or service entry (may be a plain string or a table).
type NamedEntry struct {
	Name string
}

// PackageEntry is a resolved package with its manager.
type PackageEntry struct {
	Name    string
	Manager string
}

// ServiceEntry is a resolved service with its manager.
type ServiceEntry struct {
	Name    string
	Manager string
}

// Load reads and parses a dotm.toml file.
// Paths containing ~ are expanded to the user's home directory.
func Load(path string) (*Config, error) {
	var raw map[string]any
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg := &Config{
		Managers: make(map[string]ManagerConfig),
		Groups:   make(map[string]GroupConfig),
		Prompts:  make(map[string]PromptConfig),
		Symlinks: make(map[string]string),
	}

	// Parse struct fields with known schema (dest, shell, symlinks, scripts, prompts).
	if err := decodeStructFields(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Parse managers.
	if rawManagers, ok := raw["managers"]; ok {
		mm, ok := rawManagers.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("managers must be a table")
		}
		for name, val := range mm {
			m, ok := val.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("managers.%s must be a table", name)
			}
			mgr := ManagerConfig{}
			for field, ptr := range map[string]*string{
				"check":   &mgr.Check,
				"install": &mgr.Install,
				"remove":  &mgr.Remove,
				"enable":  &mgr.Enable,
				"disable": &mgr.Disable,
			} {
				if v, ok := m[field]; ok {
					s, ok := v.(string)
					if !ok {
						return nil, fmt.Errorf("managers.%s: %s must be a string", name, field)
					}
					*ptr = s
				}
			}
			cfg.Managers[name] = mgr
		}
	}

	// Parse groups (everything except "managers", "prompts", and struct fields).
	reservedKeys := map[string]bool{
		"managers": true, "prompts": true,
		"dest": true, "shell": true, "symlinks": true, "scripts": true,
	}
	for key, val := range raw {
		if reservedKeys[key] {
			continue
		}
		group, err := parseGroup(val)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		cfg.Groups[key] = *group
	}

	cfg.Dest = expandHome(cfg.Dest)

	// Default shell to bash if not specified.
	if cfg.Shell == "" {
		cfg.Shell = "bash"
	}

	// Resolve packages and services once during load.
	cfg.resolvePackagesAndServices()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return cfg, nil
}

// decodeStructFields decodes dest, shell, symlinks, scripts from raw map.
func decodeStructFields(raw map[string]any, cfg *Config) error {
	if v, ok := raw["dest"]; ok {
		if s, ok := v.(string); ok {
			cfg.Dest = s
		}
	}
	if v, ok := raw["shell"]; ok {
		if s, ok := v.(string); ok {
			cfg.Shell = s
		}
	}
	if v, ok := raw["symlinks"]; ok {
		if m, ok := v.(map[string]any); ok {
			for k, val := range m {
				if s, ok := val.(string); ok {
					cfg.Symlinks[k] = s
				}
			}
		}
	}
	if v, ok := raw["scripts"]; ok {
		// BurntSushi decodes [[scripts]] as []map[string]any.
		var items []map[string]any
		switch arr := v.(type) {
		case []map[string]any:
			items = arr
		case []any:
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					items = append(items, m)
				}
			}
		default:
			return fmt.Errorf("scripts must be an array")
		}
		for i, m := range items {
			sc := ScriptConfig{Trigger: "always"}
			if s, ok := m["path"].(string); ok {
				sc.Path = s
			}
			if b, ok := m["template"].(bool); ok {
				sc.Template = b
			}
			if s, ok := m["trigger"].(string); ok {
				sc.Trigger = s
			}
			cfg.Scripts = append(cfg.Scripts, sc)
			_ = i // i used for error reporting if needed
		}
	}
	if v, ok := raw["prompts"]; ok {
		pm, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("prompts must be a table")
		}
		for name, val := range pm {
			p, ok := val.(map[string]any)
			if !ok {
				return fmt.Errorf("prompts.%s must be a table", name)
			}
			prompt := PromptConfig{}
			if s, ok := p["type"].(string); ok {
				prompt.Type = s
			}
			if s, ok := p["question"].(string); ok {
				prompt.Question = s
			}
			cfg.Prompts[name] = prompt
		}
	}
	return nil
}

func parseGroup(val any) (*GroupConfig, error) {
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a table")
	}

	g := &GroupConfig{}

	// Parse packages.
	if rawPkgs, ok := m["packages"]; ok {
		arr, ok := rawPkgs.([]any)
		if !ok {
			return nil, fmt.Errorf("packages must be an array")
		}
		for _, item := range arr {
			switch v := item.(type) {
			case string:
				g.Packages = append(g.Packages, NamedEntry{Name: v})
			case map[string]any:
				name, ok := v["name"].(string)
				if !ok {
					return nil, fmt.Errorf("entry must have a string 'name' field, got %v", v)
				}
				g.Packages = append(g.Packages, NamedEntry{Name: name})
			}
		}
	}

	// Parse services.
	if rawSvcs, ok := m["services"]; ok {
		arr, ok := rawSvcs.([]any)
		if !ok {
			return nil, fmt.Errorf("services must be an array")
		}
		for _, item := range arr {
			switch v := item.(type) {
			case string:
				g.Services = append(g.Services, NamedEntry{Name: v})
			case map[string]any:
				name, ok := v["name"].(string)
				if !ok {
					return nil, fmt.Errorf("entry must have a string 'name' field, got %v", v)
				}
				g.Services = append(g.Services, NamedEntry{Name: name})
			}
		}
	}

	// Reject unknown keys in group sections.
	for key := range m {
		if key != "packages" && key != "services" {
			return nil, fmt.Errorf("unknown key %q (expected 'packages' or 'services')", key)
		}
	}

	return g, nil
}

func (c *Config) validate() error {
	if c.Dest == "" && len(c.Managers) == 0 {
		return fmt.Errorf("dest is required when no managers are defined")
	}

	for name, p := range c.Prompts {
		switch p.Type {
		case "bool", "string":
		default:
			return fmt.Errorf("prompt %q: unknown type %q", name, p.Type)
		}
		if p.Question == "" {
			return fmt.Errorf("prompt %q: question is required", name)
		}
	}

	for i := range c.Scripts {
		s := &c.Scripts[i]
		if s.Path == "" {
			return fmt.Errorf("scripts[%d]: path is required", i)
		}
		if s.Trigger == "" {
			s.Trigger = "always"
		}
		switch s.Trigger {
		case "always", "on_change":
		default:
			return fmt.Errorf("scripts[%d]: unknown trigger %q", i, s.Trigger)
		}
	}

	// Validate managers.
	for name, mgr := range c.Managers {
		if mgr.Check == "" {
			return fmt.Errorf("managers.%s: check command is required", name)
		}
	}
	// Validate that groups reference known managers with required commands.
	for name, group := range c.Groups {
		mgr, ok := c.Managers[name]
		if !ok {
			return fmt.Errorf("unknown manager %q", name)
		}
		if len(group.Packages) > 0 && mgr.Install == "" {
			return fmt.Errorf("managers.%s: install command is required (has packages)", name)
		}
		if len(group.Packages) > 0 && mgr.Remove == "" {
			return fmt.Errorf("managers.%s: remove command is required (has packages)", name)
		}
		if len(group.Services) > 0 && mgr.Enable == "" {
			return fmt.Errorf("managers.%s: enable command is required (has services)", name)
		}
		if len(group.Services) > 0 && mgr.Disable == "" {
			return fmt.Errorf("managers.%s: disable command is required (has services)", name)
		}
	}

	return nil
}

// sortedGroupNames returns group names in sorted order for deterministic iteration.
func (c *Config) sortedGroupNames() []string {
	names := make([]string, 0, len(c.Groups))
	for name := range c.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolvePackagesAndServices computes resolved packages and services once.
func (c *Config) resolvePackagesAndServices() {
	names := c.sortedGroupNames()

	var pkgs []PackageEntry
	var svcs []ServiceEntry

	for _, mgrName := range names {
		group := c.Groups[mgrName]
		if len(group.Packages) > 0 {
			for _, entry := range group.Packages {
				pkgs = append(pkgs, PackageEntry{
					Name:    entry.Name,
					Manager: mgrName,
				})
			}
		}
		if len(group.Services) > 0 {
			for _, entry := range group.Services {
				svcs = append(svcs, ServiceEntry{
					Name:    entry.Name,
					Manager: mgrName,
				})
			}
		}
	}

	c.resolvedPkgs = pkgs
	c.resolvedSvcs = svcs
}

// Packages returns all resolved packages with their manager.
func (c *Config) Packages() []PackageEntry {
	return c.resolvedPkgs
}

// Services returns all resolved services with their manager.
func (c *Config) Services() []ServiceEntry {
	return c.resolvedSvcs
}

// HasPackages returns true if any packages are defined.
func (c *Config) HasPackages() bool {
	for _, group := range c.Groups {
		if len(group.Packages) > 0 {
			return true
		}
	}
	return false
}

// HasServices returns true if any services are defined.
func (c *Config) HasServices() bool {
	for _, group := range c.Groups {
		if len(group.Services) > 0 {
			return true
		}
	}
	return false
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if path == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, path[2:])
		}
	}
	return path
}
