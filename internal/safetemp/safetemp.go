package safetemp

import (
	"os"
	"path/filepath"
)

// SecureDir returns a directory suitable for temporary files that should
// not be accessible to other users. The directory is created with mode 0700
// if it does not exist.
//
// Priority:
//  1. $XDG_RUNTIME_DIR/dotm/       — typically /run/user/<uid>, mode 0700
//  2. $HOME/.local/state/dotm/tmp/ — user state directory, mode 0700
//  3. ""                           — fallback to os.TempDir() behavior
func SecureDir() string {
	dirs := secureDirs()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return dir
		}
	}
	return ""
}

func secureDirs() []string {
	var result []string

	// 1. XDG_RUNTIME_DIR — owned by user, mode 0700, cleared on logout
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		result = append(result, filepath.Join(dir, "dotm"))
	}

	// 2. User state directory — persistent, but still user-only
	if home, err := os.UserHomeDir(); err == nil {
		result = append(result, filepath.Join(home, ".local", "state", "dotm", "tmp"))
	}

	return result
}
