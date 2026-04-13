package prompt

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"dotm/internal/config"
)

// Manifest records what apply last wrote to dest.
// All paths are relative to dest (e.g. ".config/hypr/hyprland.conf").
type Manifest struct {
	Files       []string `toml:"files"`
	Directories []string `toml:"directories"`
	Symlinks    []string `toml:"symlinks"`
}

// State holds cached prompt answers, script hashes, and the deploy manifest.
type State struct {
	Data         map[string]any    `toml:"data"`
	ScriptHashes map[string]string `toml:"script_hashes"`
	Manifest     Manifest          `toml:"manifest"`
}

// stateDir returns ~/.local/state/dotm, creating it if needed.
func stateDir() (string, error) {
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

// stateFile returns the path to the state file for a given source directory.
// Each source repo gets its own state file, keyed by a hash of its
// absolute path so multiple repos don't collide.
func stateFile(sourceDir string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(abs))
	name := fmt.Sprintf("%x.toml", h[:8])
	return filepath.Join(dir, name), nil
}

// LoadState reads the state file for sourceDir.
// Returns an empty state if the file doesn't exist.
func LoadState(sourceDir string) (*State, error) {
	path, err := stateFile(sourceDir)
	if err != nil {
		return nil, err
	}

	s := &State{
		Data:         make(map[string]any),
		ScriptHashes: make(map[string]string),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}

	if _, err := toml.DecodeFile(path, s); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", path, err)
	}

	if s.Data == nil {
		s.Data = make(map[string]any)
	}
	if s.ScriptHashes == nil {
		s.ScriptHashes = make(map[string]string)
	}

	return s, nil
}

// Save writes the state to disk atomically.
func (s *State) Save(sourceDir string) error {
	path, err := stateFile(sourceDir)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := toml.NewEncoder(tmp).Encode(s); err != nil {
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

// SetManifest replaces the manifest with new lists of deployed paths.
// All paths must be relative to dest.
func (s *State) SetManifest(files, directories, symlinks []string) {
	sort.Strings(files)
	sort.Strings(directories)
	sort.Strings(symlinks)
	s.Manifest = Manifest{
		Files:       files,
		Directories: directories,
		Symlinks:    symlinks,
	}
}

// Resolve ensures all prompts defined in cfg have values in state.
// Missing values are asked interactively via r/w (typically stdin/stdout).
// Returns true if any new values were added (state needs saving).
func Resolve(cfg *config.Config, s *State, r io.Reader, w io.Writer) (bool, error) {
	if len(cfg.Prompts) == 0 {
		return false, nil
	}

	scanner := bufio.NewScanner(r)
	changed := false

	// Sort keys for deterministic prompt order.
	names := make([]string, 0, len(cfg.Prompts))
	for name := range cfg.Prompts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := cfg.Prompts[name]

		if _, exists := s.Data[name]; exists {
			continue
		}

		switch p.Type {
		case "bool":
			val, err := askBool(scanner, w, p.Question)
			if err != nil {
				return changed, err
			}
			s.Data[name] = val
			changed = true
		case "string":
			val, err := askString(scanner, w, p.Question)
			if err != nil {
				return changed, err
			}
			s.Data[name] = val
			changed = true
		}
	}

	return changed, nil
}

func askBool(scanner *bufio.Scanner, w io.Writer, question string) (bool, error) {
	for {
		if _, err := fmt.Fprintf(w, "%s [y/n]: ", question); err != nil {
			return false, err
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, err
			}
			return false, fmt.Errorf("unexpected end of input")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch answer {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		if _, err := fmt.Fprintf(w, "  please answer y or n\n"); err != nil {
			return false, err
		}
	}
}

func askString(scanner *bufio.Scanner, w io.Writer, question string) (string, error) {
	if _, err := fmt.Fprintf(w, "%s: ", question); err != nil {
		return "", err
	}
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("unexpected end of input")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// BuildData merges prompt data with built-in variables to create
// the template context.
func BuildData(s *State, sourceDir string) (map[string]any, error) {
	data := make(map[string]any, len(s.Data)+4)

	// Copy prompt values.
	for k, v := range s.Data {
		data[k] = coerceValue(v)
	}

	// Built-in variables — use fallbacks on error.
	if home, err := os.UserHomeDir(); err == nil {
		data["homeDir"] = home
	}
	if hostname, err := os.Hostname(); err == nil {
		data["hostname"] = hostname
	}
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	data["username"] = username

	abs, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("abs path %s: %w", sourceDir, err)
	}
	data["sourceDir"] = abs

	return data, nil
}

// coerceValue fixes TOML decode artifacts.
// TOML integers decode as int64; templates expect bool to stay bool.
// This is a safety net — normally values come from our own prompts.
func coerceValue(v any) any {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case string:
		// Check if a string looks like a bool (from manual edits).
		lower := strings.ToLower(val)
		if lower == "true" {
			return true
		}
		if lower == "false" {
			return false
		}
		return val
	default:
		return v
	}
}

// SetScriptHash records the hash for a script path.
func (s *State) SetScriptHash(scriptPath string, hash string) {
	s.ScriptHashes[scriptPath] = hash
}

// GetScriptHash returns the stored hash for a script path, or "" if none.
func (s *State) GetScriptHash(scriptPath string) string {
	return s.ScriptHashes[scriptPath]
}

// HashContent returns a hex-encoded SHA-256 of content.
func HashContent(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", h)
}

// FormatStateFile returns a human-readable identifier for the state file
// (for init output).
func FormatStateFile(sourceDir string) string {
	path, err := stateFile(sourceDir)
	if err != nil {
		return "(unknown)"
	}

	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(path, home) {
		return path
	}
	return "~" + path[len(home):]
}

// ResetPrompt removes a cached prompt value so it will be asked again.
func (s *State) ResetPrompt(name string) {
	delete(s.Data, name)
}

// FormatPromptValue returns a displayable representation of a prompt value.
func FormatPromptValue(v any) string {
	switch val := v.(type) {
	case bool:
		return strconv.FormatBool(val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
