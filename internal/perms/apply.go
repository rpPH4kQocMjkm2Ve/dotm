package perms

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PermAction is a computed permission action for a single target path.
type PermAction struct {
	Path  string
	Mode  int    // -1 if not set
	Owner string // "" if not set
	Group string // "" if not set
	Rule  *PermRule
}

// ComputeActions determines what permission changes to apply.
// Last matching rule wins. isDirFunc is injectable for testing.
func ComputeActions(
	rules []PermRule,
	managedPaths []string,
	destDir string,
	isDirFunc func(string) bool,
) []PermAction {
	if isDirFunc == nil {
		isDirFunc = isDir
	}

	destDir = strings.TrimRight(destDir, "/")
	prefix := destDir + "/"

	// Pre-compile all patterns.
	type compiled struct {
		rule  *PermRule
		regex func(string) bool
	}
	matchers := make([]compiled, len(rules))
	for i := range rules {
		r := &rules[i]
		pattern := r.Pattern
		matchers[i] = compiled{
			rule:  r,
			regex: func(path string) bool { return MatchGlob(pattern, path) },
		}
	}

	var actions []PermAction

	for _, target := range managedPaths {
		if target == destDir {
			continue
		}
		if !strings.HasPrefix(target, prefix) {
			continue
		}
		rel := target[len(prefix):]
		dir := isDirFunc(target)

		var matched *PermRule
		for _, m := range matchers {
			if m.rule.DirOnly && !dir {
				continue
			}
			if !m.rule.DirOnly && dir {
				continue
			}
			if m.regex(rel) {
				matched = m.rule
			}
		}

		if matched == nil {
			continue
		}

		mode := -1
		if matched.Mode != "" {
			parsed, _ := strconv.ParseInt(matched.Mode, 8, 32)
			mode = int(parsed)
		}

		actions = append(actions, PermAction{
			Path:  target,
			Mode:  mode,
			Owner: matched.Owner,
			Group: matched.Group,
			Rule:  matched,
		})
	}

	return actions
}

// ApplyActions applies permission actions to the filesystem.
// Returns (allOK, errors). Continues on individual failures.
func ApplyActions(actions []PermAction, dryRun bool) (bool, []string) {
	var errors []string

	for _, a := range actions {
		if dryRun {
			modeStr := "-"
			if a.Mode >= 0 {
				modeStr = fmt.Sprintf("%04o", a.Mode)
			}
			ownerStr := a.Owner
			if ownerStr == "" {
				ownerStr = "-"
			}
			groupStr := a.Group
			if groupStr == "" {
				groupStr = "-"
			}
			fmt.Printf("%s %s:%s %s\n", modeStr, ownerStr, groupStr, a.Path)
			continue
		}

		if a.Mode >= 0 {
			if err := os.Chmod(a.Path, os.FileMode(a.Mode)); err != nil {
				errors = append(errors, fmt.Sprintf("chmod %04o %s: %v", a.Mode, a.Path, err))
			}
		}

		if a.Owner != "" || a.Group != "" {
			uid := -1
			gid := -1

			if a.Owner != "" {
				u, err := user.Lookup(a.Owner)
				if err != nil {
					errors = append(errors, fmt.Sprintf("chown %s:%s %s: %v", a.Owner, a.Group, a.Path, err))
					continue
				}
				uid, _ = strconv.Atoi(u.Uid)
			}

			if a.Group != "" {
				g, err := user.LookupGroup(a.Group)
				if err != nil {
					errors = append(errors, fmt.Sprintf("chown %s:%s %s: %v", a.Owner, a.Group, a.Path, err))
					continue
				}
				gid, _ = strconv.Atoi(g.Gid)
			}

			if err := os.Chown(a.Path, uid, gid); err != nil {
				errors = append(errors, fmt.Sprintf("chown %s:%s %s: %v", a.Owner, a.Group, a.Path, err))
			}
		}
	}

	return len(errors) == 0, errors
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CollectManagedPaths walks destDir and returns all paths that exist.
// Used to build the managed paths list for ComputeActions when
// the caller doesn't have an explicit list.
func CollectManagedPaths(destDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(destDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the root directory itself.
		if path == destDir {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

// FileMode returns the current permission bits of a file.
// Returns -1 on error.
func FileMode(path string) int {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return -1
	}
	return int(st.Mode & 0o7777)
}
