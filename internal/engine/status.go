package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// HasProblems returns true if there are any non-clean entries.
func (r *StatusReport) HasProblems() bool {
	for _, e := range r.Entries {
		if e.Status != StatusClean {
			return true
		}
	}
	return false
}

// Status compares rendered source, dest, and manifest to produce a report.
func (e *Engine) Status() (*StatusReport, error) {
	report := &StatusReport{}

	// 1. Build set of current source files (relative to dest).
	sourceFiles, err := e.collectSourceFiles()
	if err != nil {
		return nil, fmt.Errorf("collect source files: %w", err)
	}
	sourceSet := make(map[string]bool, len(sourceFiles))
	for _, rel := range sourceFiles {
		sourceSet[rel] = true
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

		// srcPath may be empty if source was not found (shouldn't happen
		// since we just collected it, but guard anyway).
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
			continue // already handled as a file (unlikely but safe)
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

	// Sort entries by path for stable output.
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].RelPath < report.Entries[j].RelPath
	})

	return report, nil
}

// collectSourceFiles walks files/ and returns all non-ignored file paths
// relative to dest (with .tmpl suffix stripped).
func (e *Engine) collectSourceFiles() ([]string, error) {
	if _, err := os.Stat(e.filesDir); os.IsNotExist(err) {
		return nil, nil
	}

	var result []string
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
			return nil // only track files, not directories
		}

		result = append(result, destRel)
		return nil
	})

	return result, err
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
func PrintReport(report *StatusReport, verbose bool) {
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
				color = "\033[32m"
				reset = "\033[0m"
			case StatusModified:
				color = "\033[33m"
				reset = "\033[0m"
			case StatusMissing:
				color = "\033[31m"
				reset = "\033[0m"
			case StatusOrphan:
				color = "\033[35m"
				reset = "\033[0m"
			}
		}

		label := FormatStatus(entry.Status)
		padded := label + strings.Repeat(" ", 10-len(label))
		fmt.Printf("  %s%s%s %s\n", color, padded, reset, entry.RelPath)
	}
}
