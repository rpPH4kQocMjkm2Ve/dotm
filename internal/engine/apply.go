package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dotm/internal/config"
	"dotm/internal/ignore"
	"dotm/internal/perms"
	"dotm/internal/prompt"
	"dotm/internal/tmpl"
)

// Engine holds the resolved state needed to apply, diff, or status.
type Engine struct {
	cfg       *config.Config
	state     *prompt.State
	data      map[string]any
	sourceDir string
	filesDir  string
	ig        *ignore.Ignore
	dryRun    bool
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
		filesDir:  filepath.Join(sourceDir, "files"),
		ig:        ig,
		dryRun:    dryRun,
	}, nil
}

// Apply walks files/, copies/renders to dest, creates symlinks,
// applies perms, runs scripts, and records the manifest.
func (e *Engine) Apply() error {
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

	// 5. Record manifest (skip on dry run — nothing was actually written).
	if !e.dryRun {
		e.recordManifest(written)
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
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, rel)
		} else {
			files = append(files, rel)
		}
	}

	// Symlinks defined in config.
	var symlinks []string
	for linkRel := range e.cfg.Symlinks {
		symlinks = append(symlinks, linkRel)
	}

	e.state.SetManifest(files, dirs, symlinks)
}

// Diff shows a unified diff between rendered source and current dest.
func (e *Engine) Diff() error {
	if _, err := os.Stat(e.filesDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(e.filesDir, func(path string, info os.FileInfo, err error) error {
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

		// Get rendered source content.
		srcContent, err := e.fileContent(path, rel)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}

		// Get current dest content.
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
	for linkRel, targetTmpl := range e.cfg.Symlinks {
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
			e.state.SetScriptHash(sc.Path, hash)
		}

		if e.dryRun {
			fmt.Printf("run %s\n", sc.Path)
			continue
		}

		if err := execScript(content, e.cfg.Shell); err != nil {
			return fmt.Errorf("script %s: %w", sc.Path, err)
		}
	}

	return nil
}

// execScript writes content to a temp file and executes it with the configured shell.
func execScript(content []byte, shell string) error {
	tmp, err := os.CreateTemp("", "dotm-script-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

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
	defer os.Remove(oldTmp)

	newTmp, err := writeTmp("dotm-new-", newContent)
	if err != nil {
		return err
	}
	defer os.Remove(newTmp)

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
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func stripTmplSuffix(path string) string {
	return strings.TrimSuffix(path, ".tmpl")
}
