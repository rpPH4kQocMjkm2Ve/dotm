package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"dotm/internal/config"
	"dotm/internal/ignore"
	"dotm/internal/manifest"
	"dotm/internal/perms"
	"dotm/internal/prompt"
	"dotm/internal/safetemp"
	"dotm/internal/tmpl"
)


// Engine holds the resolved state needed to apply, diff, or status.
type Engine struct {
	cfg       *config.Config
	state     *prompt.State
	data      map[string]any
	sourceDir string
	configDir string
	filesDir  string
	ig        *ignore.Ignore
	dryRun    bool
	tmplCache map[string]*template.Template
}

// New creates an Engine from a loaded config, resolved state, and source dir.
func New(cfg *config.Config, state *prompt.State, sourceDir string, dryRun bool) (*Engine, error) {
	data, err := prompt.BuildData(state, sourceDir)
	if err != nil {
		return nil, fmt.Errorf("build template data: %w", err)
	}

	ig, err := ignore.Load(sourceDir, data)
	if err != nil {
		return nil, fmt.Errorf("load ignore: %w", err)
	}

	return &Engine{
		cfg:       cfg,
		state:     state,
		data:      data,
		sourceDir: sourceDir,
		configDir: sourceDir,
		filesDir:  filepath.Join(sourceDir, "files"),
		ig:        ig,
		dryRun:    dryRun,
	}, nil
}

// Apply walks files/, copies/renders to dest, creates symlinks,
// applies perms, runs scripts, and records the manifest.
func (e *Engine) Apply(scope ScopeBit) error {
	// Steps 1-5: file operations (only when dest is configured and file scope).
	if scope.Has(ScopeFiles) && e.cfg.Dest != "" {
		// 1. Walk files/ and copy/render.
		written, err := e.walkAndWrite()
		if err != nil {
			return fmt.Errorf("apply files: %w", err)
		}

		// 2. Create symlinks.
		if err := e.applySymlinks(); err != nil {
			return fmt.Errorf("apply symlinks: %w", err)
		}

		// 3. Apply permissions.
		if err := e.applyPerms(written); err != nil {
			return fmt.Errorf("apply perms: %w", err)
		}

		// 4. Run scripts.
		if err := e.runScripts(); err != nil {
			return fmt.Errorf("run scripts: %w", err)
		}

		// 5. Record file manifest (skip on dry run).
		if !e.dryRun {
			e.recordManifest(written)
		}
	}

	// 6. Apply packages and services (if managers are defined).
	if len(e.cfg.Managers) > 0 && (scope.Has(ScopePkgs) || scope.Has(ScopeServices)) {
		var pkgEntries []manifest.PackageEntry
		var svcEntries []manifest.ServiceEntry
		var allRemoveErrs []error

		if scope.Has(ScopePkgs) {
			var pkgErrs []error
			pkgEntries, pkgErrs = e.applyPackages(e.dryRun)
			allRemoveErrs = append(allRemoveErrs, pkgErrs...)
		}
		if scope.Has(ScopeServices) {
			var svcErrs []error
			svcEntries, svcErrs = e.applyServices(e.dryRun)
			allRemoveErrs = append(allRemoveErrs, svcErrs...)
		}

		// Save package manifest (only if no errors and not dry run).
		if len(allRemoveErrs) == 0 && !e.dryRun {
			if err := e.savePkgManifest(pkgEntries, svcEntries); err != nil {
				return fmt.Errorf("save package manifest: %w", err)
			}
		}

		if len(allRemoveErrs) > 0 {
			return fmt.Errorf("failed to install/remove/enable/disable %d item(s): fix the errors and re-run apply", len(allRemoveErrs))
		}
	}

	return nil
}

// recordManifest saves the list of deployed paths into state.
func (e *Engine) recordManifest(writtenAbs []string) {
	var files, dirs []string
	dest := filepath.Clean(e.cfg.Dest)

	for _, abs := range writtenAbs {
		rel, err := filepath.Rel(dest, abs)
		if err != nil || rel == "." {
			continue
		}

		info, err := os.Lstat(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot stat %s: %v\n", abs, err)
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, rel)
		} else {
			files = append(files, rel)
		}
	}

	// Symlinks defined in config.
	symlinkKeys := make([]string, 0, len(e.cfg.Symlinks))
	for linkRel := range e.cfg.Symlinks {
		symlinkKeys = append(symlinkKeys, linkRel)
	}
	sort.Strings(symlinkKeys)
	symlinks := make([]string, 0, len(symlinkKeys))
	symlinks = append(symlinks, symlinkKeys...)

	e.state.SetManifest(files, dirs, symlinks)
}

// Diff shows a unified diff between rendered source and current dest,
// and package/service changes.
func (e *Engine) Diff(scope ScopeBit) error {
	if scope.Has(ScopeFiles) {
		if _, err := os.Stat(e.filesDir); err == nil {
			err := filepath.Walk(e.filesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(e.filesDir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}

			destRel := stripTmplSuffix(rel)

			if e.ig.Match(destRel) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				return nil
			}

			destPath := filepath.Join(e.cfg.Dest, destRel)

			srcContent, err := e.fileContent(path, rel)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}

			destContent, err := os.ReadFile(destPath)
			if err != nil {
				if os.IsNotExist(err) {
					destContent = nil
				} else {
					return err
				}
			}

			if bytes.Equal(srcContent, destContent) {
				return nil
			}

			return showDiff(destPath, destRel, destContent, srcContent)
		})
		if err != nil {
			return err
		}
		}
	}

	if len(e.cfg.Managers) > 0 {
		if scope.Has(ScopePkgs) {
			e.diffPackages()
		}
		if scope.Has(ScopeServices) {
			e.diffServices()
		}
	}

	return nil
}

// walkAndWrite processes all files in files/ and writes them to dest.
// Returns all written absolute paths (files and directories).
func (e *Engine) walkAndWrite() ([]string, error) {
	if _, err := os.Stat(e.filesDir); os.IsNotExist(err) {
		return nil, nil
	}

	var written []string

	err := filepath.Walk(e.filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(e.filesDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		destRel := stripTmplSuffix(rel)

		if e.ig.Match(destRel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(e.cfg.Dest, destRel)

		if info.IsDir() {
			if e.dryRun {
				fmt.Printf("mkdir %s\n", destPath)
			} else {
				if err := os.MkdirAll(destPath, 0o755); err != nil {
					return err
				}
			}
			written = append(written, destPath)
			return nil
		}

		content, err := e.fileContent(path, rel)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}

		if e.dryRun {
			fmt.Printf("write %s (%d bytes)\n", destPath, len(content))
			written = append(written, destPath)
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		// Skip write if content is identical.
		existing, err := os.ReadFile(destPath)
		if err == nil && bytes.Equal(existing, content) {
			written = append(written, destPath)
			return nil
		}

		// Write with restrictive permissions initially.
		// Correct permissions are applied later by applyPerms.
		// This avoids a window where sensitive files are world-readable.
		if err := os.WriteFile(destPath, content, 0o600); err != nil {
			return err
		}

		written = append(written, destPath)
		return nil
	})

	return written, err
}

// fileContent returns the content for a source file.
// Templates (.tmpl suffix) are rendered; other files are read as-is.
func (e *Engine) fileContent(srcPath, rel string) ([]byte, error) {
	if strings.HasSuffix(rel, ".tmpl") {
		return tmpl.RenderFile(srcPath, e.data)
	}
	return os.ReadFile(srcPath)
}

// applySymlinks creates symlinks defined in [symlinks].
func (e *Engine) applySymlinks() error {
	// Sort keys for deterministic order.
	links := make([]string, 0, len(e.cfg.Symlinks))
	for linkRel := range e.cfg.Symlinks {
		links = append(links, linkRel)
	}
	sort.Strings(links)

	for _, linkRel := range links {
		targetTmpl := e.cfg.Symlinks[linkRel]
		// Target may contain template expressions like {{ .homeDir }}.
		rendered, err := tmpl.Render(targetTmpl, "symlink:"+linkRel, e.data)
		if err != nil {
			return fmt.Errorf("symlink %s: render target: %w", linkRel, err)
		}
		targetStr := strings.TrimSpace(string(rendered))
		linkPath := filepath.Join(e.cfg.Dest, linkRel)

		if e.dryRun {
			fmt.Printf("symlink %s -> %s\n", linkPath, targetStr)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
			return err
		}

		// Check if symlink already points to the correct target.
		if existing, err := os.Lstat(linkPath); err == nil {
			if existing.Mode()&os.ModeSymlink != 0 {
				current, err := os.Readlink(linkPath)
				if err == nil && current == targetStr {
					continue
				}
			}
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("symlink %s: remove existing: %w", linkPath, err)
			}
		}

		if err := os.Symlink(targetStr, linkPath); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", linkPath, targetStr, err)
		}
	}

	return nil
}

// applyPerms loads the perms file and applies permission rules
// to written paths.
func (e *Engine) applyPerms(writtenPaths []string) error {
	permsPath := filepath.Join(e.sourceDir, "perms")
	content, err := os.ReadFile(permsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No perms file — apply default permissions to all written paths.
			// Files were initially written with 0o600 for security;
			// this lifts them to the previous defaults (0o644 for files,
			// 0o755 for directories).
			for _, p := range writtenPaths {
				if !e.dryRun {
					info, statErr := os.Stat(p)
					if statErr != nil {
						fmt.Fprintf(os.Stderr, "WARN: cannot stat %s: %v\n", p, statErr)
						continue
					}
					var mode os.FileMode
					if info.IsDir() {
						mode = 0o755
					} else {
						mode = 0o644
					}
					if err := os.Chmod(p, mode); err != nil {
						fmt.Fprintf(os.Stderr, "WARN: chmod %s: %v\n", p, err)
					}
				}
			}
			return nil
		}
		return err
	}

	rules, err := perms.ParseRules(string(content))
	if err != nil {
		return fmt.Errorf("perms: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}

	actions := perms.ComputeActions(rules, writtenPaths, e.cfg.Dest, nil)
	if len(actions) == 0 {
		return nil
	}

	ok, errors := perms.ApplyActions(actions, e.dryRun)
	if !ok {
		for _, msg := range errors {
			fmt.Fprintf(os.Stderr, "perms: %s\n", msg)
		}
		return fmt.Errorf("perms: %d errors", len(errors))
	}

	return nil
}

// runScripts executes lifecycle scripts defined in [[scripts]].
func (e *Engine) runScripts() error {
	for _, sc := range e.cfg.Scripts {
		scriptPath := filepath.Join(e.sourceDir, sc.Path)

		var content []byte
		var err error
		if sc.Template {
			content, err = tmpl.RenderFile(scriptPath, e.data)
		} else {
			content, err = os.ReadFile(scriptPath)
		}
		if err != nil {
			return fmt.Errorf("script %s: %w", sc.Path, err)
		}

		// For on_change: skip if content hash hasn't changed.
		if sc.Trigger == "on_change" {
			hash := prompt.HashContent(content)
			if e.state.GetScriptHash(sc.Path) == hash {
				continue
			}
		}

		if e.dryRun {
			fmt.Printf("run %s\n", sc.Path)
			continue
		}

		if err := execScript(content, e.cfg.Shell); err != nil {
			return fmt.Errorf("script %s: %w", sc.Path, err)
		}

		// Record hash AFTER successful execution so a failed script
		// will be retried on the next run.
		if sc.Trigger == "on_change" {
			e.state.SetScriptHash(sc.Path, prompt.HashContent(content))
		}
	}

	return nil
}

// execScript writes content to a temp file in a secure directory and executes
// it with the configured shell. Uses XDG_RUNTIME_DIR or user state directory
// to prevent symlink race attacks in /tmp.
func execScript(content []byte, shell string) error {
	// Validate shell is a known safe path.
	if !isValidShell(shell) {
		return fmt.Errorf("invalid shell %q: must be an absolute path to a known shell", shell)
	}

	dir := safetemp.SecureDir()

	tmp, err := os.CreateTemp(dir, "dotm-script-*.sh")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmp.Name(), 0o700); err != nil {
		return err
	}

	cmd := exec.Command(shell, tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// showDiff runs diff(1) between old and new content.
func showDiff(destPath, label string, oldContent, newContent []byte) error {
	oldTmp, err := writeTmp("dotm-old-", oldContent)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(oldTmp) }()

	newTmp, err := writeTmp("dotm-new-", newContent)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(newTmp) }()

	cmd := exec.Command("diff", "--color=auto", "-u",
		"--label", "dest:"+label,
		"--label", "source:"+label,
		oldTmp, newTmp,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// diff exits 1 when files differ — not an error for us.
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}
	return nil
}

func writeTmp(prefix string, content []byte) (string, error) {
	if content == nil {
		content = []byte{}
	}

	dir := safetemp.SecureDir()
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func stripTmplSuffix(path string) string {
	return strings.TrimSuffix(path, ".tmpl")
}

// isValidShell checks if the shell is a known safe shell path.
// Accepts absolute paths like /bin/bash, /bin/sh, /bin/zsh or bare names
// that resolve to known shells (bash, sh, zsh, fish).
func isValidShell(shell string) bool {
	// Known safe shell paths.
	knownShells := map[string]bool{
		"/bin/sh":     true,
		"/bin/bash":   true,
		"/bin/zsh":    true,
		"/bin/fish":   true,
		"/usr/bin/sh":     true,
		"/usr/bin/bash":   true,
		"/usr/bin/zsh":    true,
		"/usr/bin/fish":   true,
		"sh":    true,
		"bash":  true,
		"zsh":   true,
		"fish":  true,
	}

	if knownShells[shell] {
		return true
	}

	// If it's an absolute path, check if it exists, is a regular file, and is executable.
	if filepath.IsAbs(shell) {
		info, err := os.Stat(shell)
		if err != nil {
			return false
		}
		return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
	}

	return false
}
