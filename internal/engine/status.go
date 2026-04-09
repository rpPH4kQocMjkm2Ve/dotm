package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ANSI color codes for terminal output.
const (
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiPurple = "\033[35m"
	ansiReset  = "\033[0m"
)

// FileStatus represents the state of a single managed file.
type FileStatus int

const (
	StatusClean    FileStatus = iota // dest matches rendered source
	StatusModified                   // dest exists but differs from source
	StatusMissing                    // in source but not in dest
	StatusOrphan                     // in manifest but no longer in source
)

// StatusEntry is one file in the status report.
type StatusEntry struct {
	RelPath string
	Status  FileStatus
}

// StatusReport holds the full result of a status check.
type StatusReport struct {
	Entries []StatusEntry
	// Package/service problems (reported separately, not in Entries
	// to avoid duplicate printing — packages/services are printed
	// directly by statusPackages/statusServices).
	PkgHasProblems  bool
	SvcHasProblems  bool
	PkgOrSvcPrinted bool // true if any package/service output was printed
}

// Counts returns the number of entries per status.
func (r *StatusReport) Counts() (clean, modified, missing, orphan int) {
	for _, e := range r.Entries {
		switch e.Status {
		case StatusClean:
			clean++
		case StatusModified:
			modified++
		case StatusMissing:
			missing++
		case StatusOrphan:
			orphan++
		}
	}
	return
}

// HasProblems returns true if there are any non-clean entries or
// package/service problems.
func (r *StatusReport) HasProblems() bool {
	for _, e := range r.Entries {
		if e.Status != StatusClean {
			return true
		}
	}
	return r.PkgHasProblems || r.SvcHasProblems
}

// Status compares rendered source, dest, and manifest to produce a report.
func (e *Engine) Status(scope ScopeBit, verbose bool) (*StatusReport, error) {
	report := &StatusReport{}

	// 1. Build set of current source files and directories (relative to dest).
	if scope.Has(ScopeFiles) {
		sourceFiles, sourceDirs, err := e.collectSourcePaths()
		if err != nil {
			return nil, fmt.Errorf("collect source paths: %w", err)
		}
		sourceSet := make(map[string]bool, len(sourceFiles))
		for _, rel := range sourceFiles {
			sourceSet[rel] = true
		}
		sourceDirSet := make(map[string]bool, len(sourceDirs))
		for _, rel := range sourceDirs {
			sourceDirSet[rel] = true
		}

		// 2. For each source file, compare with dest.
		for _, rel := range sourceFiles {
			destPath := filepath.Join(e.cfg.Dest, rel)
			srcPath := e.findSourceFile(rel)

			destContent, err := os.ReadFile(destPath)
			if err != nil {
				if os.IsNotExist(err) {
					report.Entries = append(report.Entries, StatusEntry{rel, StatusMissing})
					continue
				}
				return nil, fmt.Errorf("read dest %s: %w", destPath, err)
			}

			if srcPath == "" {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusMissing})
				continue
			}

			srcRel, err := filepath.Rel(e.filesDir, srcPath)
			if err != nil {
				srcRel = rel
			}
			srcContent, err := e.fileContent(srcPath, srcRel)
			if err != nil {
				return nil, fmt.Errorf("render %s: %w", rel, err)
			}

			if bytes.Equal(srcContent, destContent) {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusClean})
			} else {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusModified})
			}
		}

		// 3. Check symlinks from config.
		for linkRel := range e.cfg.Symlinks {
			if sourceSet[linkRel] {
				continue
			}
			sourceSet[linkRel] = true

			linkPath := filepath.Join(e.cfg.Dest, linkRel)
			if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
				report.Entries = append(report.Entries, StatusEntry{linkRel, StatusMissing})
			} else {
				report.Entries = append(report.Entries, StatusEntry{linkRel, StatusClean})
			}
		}

		// 4. Find orphans: in manifest but not in current source.
		for _, rel := range e.state.Manifest.Files {
			if sourceSet[rel] {
				continue
			}
			destPath := filepath.Join(e.cfg.Dest, rel)
			if _, err := os.Stat(destPath); err == nil {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusOrphan})
			}
		}
		for _, rel := range e.state.Manifest.Symlinks {
			if sourceSet[rel] {
				continue
			}
			linkPath := filepath.Join(e.cfg.Dest, rel)
			if _, err := os.Lstat(linkPath); err == nil {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusOrphan})
			}
		}
		for _, rel := range e.state.Manifest.Directories {
			if sourceDirSet[rel] {
				continue
			}
			destPath := filepath.Join(e.cfg.Dest, rel)
			if info, err := os.Stat(destPath); err == nil && info.IsDir() {
				report.Entries = append(report.Entries, StatusEntry{rel, StatusOrphan})
			}
		}
	}

	// 5. Check packages and services (if managers are defined).
	if len(e.cfg.Managers) > 0 && (scope.Has(ScopePkgs) || scope.Has(ScopeServices)) {
		prevPkgs, _ := e.loadPkgManifest()
		if scope.Has(ScopePkgs) {
			if err := e.statusPackages(report, prevPkgs, verbose); err != nil {
				return nil, fmt.Errorf("status packages: %w", err)
			}
		}
		if scope.Has(ScopeServices) {
			if err := e.statusServices(report, prevPkgs, verbose); err != nil {
				return nil, fmt.Errorf("status services: %w", err)
			}
		}
	}

	// Sort entries by path for stable output.
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].RelPath < report.Entries[j].RelPath
	})

	return report, nil
}

// collectSourcePaths walks files/ and returns all non-ignored file and directory paths
// relative to dest (with .tmpl suffix stripped).
func (e *Engine) collectSourcePaths() (files, dirs []string, err error) {
	if _, err := os.Stat(e.filesDir); os.IsNotExist(err) {
		return nil, nil, nil
	}

	err = filepath.Walk(e.filesDir, func(path string, info os.FileInfo, err error) error {
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
			dirs = append(dirs, destRel)
			return nil
		}

		files = append(files, destRel)
		return nil
	})

	return files, dirs, err
}

// findSourceFile locates the source file in files/ for a given dest-relative path.
// Checks both plain and .tmpl variants.
func (e *Engine) findSourceFile(destRel string) string {
	// Try exact path first.
	plain := filepath.Join(e.filesDir, destRel)
	if _, err := os.Stat(plain); err == nil {
		return plain
	}

	// Try with .tmpl suffix.
	tmplPath := filepath.Join(e.filesDir, destRel+".tmpl")
	if _, err := os.Stat(tmplPath); err == nil {
		return tmplPath
	}

	return ""
}

// FormatStatus returns the human-readable label for a FileStatus.
func FormatStatus(s FileStatus) string {
	switch s {
	case StatusClean:
		return "clean"
	case StatusModified:
		return "modified"
	case StatusMissing:
		return "missing"
	case StatusOrphan:
		return "orphan"
	default:
		return "unknown"
	}
}

// PrintReport writes the status report to stdout.
// If verbose is false, only non-clean entries are shown.
func PrintReport(report *StatusReport, verbose bool, scope ScopeBit) {
	// Separate file entries from package/service output.
	if report.PkgOrSvcPrinted && len(report.Entries) > 0 {
		fmt.Println()
	}

	// Print "Files:" header if there are file entries.
	if scope.Has(ScopeFiles) && len(report.Entries) > 0 {
		showHeader := verbose
		if !verbose {
			for _, entry := range report.Entries {
				if entry.Status != StatusClean {
					showHeader = true
					break
				}
			}
		}
		if showHeader {
			fmt.Println("Files:")
		}
	}

	stat, err := os.Stdout.Stat()
	isTerminal := err == nil && (stat.Mode()&os.ModeCharDevice) != 0

	for _, entry := range report.Entries {
		if !verbose && entry.Status == StatusClean {
			continue
		}

		var color string
		var reset string
		if isTerminal {
			switch entry.Status {
			case StatusClean:
				color = ansiGreen
				reset = ansiReset
			case StatusModified:
				color = ansiYellow
				reset = ansiReset
			case StatusMissing:
				color = ansiRed
				reset = ansiReset
			case StatusOrphan:
				color = ansiPurple
				reset = ansiReset
			}
		}

		label := FormatStatus(entry.Status)
		padded := label + strings.Repeat(" ", 10-len(label))
		fmt.Printf("  %s%s%s %s\n", color, padded, reset, entry.RelPath)
	}
}
