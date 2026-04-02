package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// Load reads and parses a dotm.toml file.
// Paths containing ~ are expanded to the user's home directory.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg.Dest = expandHome(cfg.Dest)

	// Default shell to bash if not specified.
	if cfg.Shell == "" {
		cfg.Shell = "bash"
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Dest == "" {
		return fmt.Errorf("dest is required")
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

	return nil
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
