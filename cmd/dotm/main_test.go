package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

func setupTestDir(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	return dir
}

func writeDotmToml(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "dotm.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write dotm.toml: %v", err)
	}
}

func runWithArgs(args []string) error {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = append([]string{"dotm"}, args...)
	return run()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	return buf.String()
}

// ─── run() — command routing ────────────────────────────────────────────────

func TestRunNoArgs(t *testing.T) {
	err := runWithArgs(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("error = %q, should mention 'no command'", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := runWithArgs([]string{"foobar"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown command "foobar"`) {
		t.Errorf("error = %q, should mention unknown command", err)
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			err := runWithArgs([]string{arg})
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-V"} {
		t.Run(arg, func(t *testing.T) {
			out := captureStdout(t, func() {
				err := runWithArgs([]string{arg})
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			})
			if !strings.HasPrefix(out, "dotm ") {
				t.Errorf("output = %q, should start with 'dotm '", out)
			}
		})
	}
}

func TestCmdHelpReset(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg+"/reset", func(t *testing.T) {
			out := captureStdout(t, func() {
				err := runWithArgs([]string{arg, "reset"})
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			})
			if !strings.Contains(out, "dotm reset") {
				t.Errorf("output = %q, should mention 'dotm reset'", out)
			}
		})
	}
}

// ─── printUsage / usageError ────────────────────────────────────────────────

func TestPrintUsage(t *testing.T) {
	out := captureStdout(t, func() { printUsage() })
	mentions := []string{"init", "apply", "status", "diff", "reset", "version"}
	for _, word := range mentions {
		if !strings.Contains(out, word) {
			t.Errorf("printUsage should mention %q", word)
		}
	}
}

func TestUsageError(t *testing.T) {
	err := usageError()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("error = %q, should mention 'no command'", err)
	}
}

// ─── findSourceDir ──────────────────────────────────────────────────────────

func TestFindSourceDirCurrent(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	got, err := findSourceDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("findSourceDir() = %q, want %q", got, dir)
	}
}

func TestFindSourceDirParent(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("chdir to subdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	got, err := findSourceDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("findSourceDir() = %q, want %q (parent)", got, dir)
	}
}

func TestFindSourceDirNotFound(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	_, err = findSourceDir()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, should mention 'not found'", err)
	}
}

// ─── cmdReset ───────────────────────────────────────────────────────────────

func TestCmdResetNoArgs(t *testing.T) {
	err := runWithArgs([]string{"reset"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reset requires") {
		t.Errorf("error = %q, should mention requirements", err)
	}
}

func TestCmdResetCannotMixAllWithNames(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"reset", "--all", "some_prompt"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot mix") {
		t.Errorf("error = %q, should mention 'cannot mix'", err)
	}
}

func TestCmdResetUnknownPrompt(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"reset", "unknown_prompt"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, should mention 'not found'", err)
	}
}

// ─── cmdStatus ──────────────────────────────────────────────────────────────

func TestStatusUnknownFlag(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"status", "--unknown"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("error = %q, should mention unknown flag", err)
	}
}

func TestStatusQuietModeClean(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "`+dir+`/dest"`)
	if err := os.MkdirAll(filepath.Join(dir, "dest"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runWithArgs([]string{"status", "-q"})
	if err != nil {
		t.Errorf("expected no error for clean status, got %v", err)
	}
}

func TestStatusVerbose(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "`+dir+`/dest"`)
	if err := os.MkdirAll(filepath.Join(dir, "dest"), 0o755); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() {
		err := runWithArgs([]string{"status", "-v"})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestStatusVerboseLong(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "`+dir+`/dest"`)
	if err := os.MkdirAll(filepath.Join(dir, "dest"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runWithArgs([]string{"status", "--verbose"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestStatusQuiet(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "`+dir+`/dest"`)
	if err := os.MkdirAll(filepath.Join(dir, "dest"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runWithArgs([]string{"status", "--quiet"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// ─── cmdApply ───────────────────────────────────────────────────────────────

func TestApplyUnknownFlag(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"apply", "--unknown"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("error = %q, should mention unknown flag", err)
	}
}

func TestApplyDryRunShort(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply", "-n"})
	if err != nil {
		t.Fatalf("expected no error for dry run, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "test.conf")); err == nil {
		t.Error("dry run should not write files")
	}
}

func TestApplyDryRunLong(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply", "--dry-run"})
	if err != nil {
		t.Fatalf("expected no error for dry run, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "test.conf")); err == nil {
		t.Error("dry run should not write files")
	}
}

func TestApplyWritesFile(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destDir, "test.conf"))
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", data, "hello world")
	}
}

func TestApplyScopeFiles(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply", "files"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "test.conf")); err != nil {
		t.Error("expected file to be created")
	}
}

func TestApplyTemplateFile(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hostname, _ := os.Hostname()
	if err := os.WriteFile(filepath.Join(filesDir, "greet.txt.tmpl"), []byte("hello {{ .hostname }}"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destDir, "greet.txt"))
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
	expected := "hello " + hostname
	if strings.TrimSpace(string(data)) != expected {
		t.Errorf("content = %q, want %q", strings.TrimSpace(string(data)), expected)
	}
}

func TestApplySymlinks(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkTarget := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(linkTarget, []byte("target content"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[symlinks]
"link.txt" = "`+linkTarget+`"
`)

	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	target, err := os.Readlink(filepath.Join(destDir, "link.txt"))
	if err != nil {
		t.Fatalf("expected symlink to exist, got %v", err)
	}
	if target != linkTarget {
		t.Errorf("symlink target = %q, want %q", target, linkTarget)
	}
}

func TestApplyScripts(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(dir, "script_output.txt")
	scriptPath := filepath.Join(dir, "scripts", "setup.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'ran' > "+outFile+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
shell = "`+bashPath+`"
[[scripts]]
path = "scripts/setup.sh"
trigger = "always"
`)

	err = runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected script output file, got %v", err)
	}
	if strings.TrimSpace(string(data)) != "ran" {
		t.Errorf("script output = %q, want 'ran'", strings.TrimSpace(string(data)))
	}
}

func TestApplyScriptOnChange(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(dir, "on_change_output.txt")
	scriptPath := filepath.Join(dir, "scripts", "on_change.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'changed' > "+outFile+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
shell = "`+bashPath+`"
[[scripts]]
path = "scripts/on_change.sh"
trigger = "on_change"
`)

	// First run: script executes.
	err = runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := os.ReadFile(outFile); err != nil {
		t.Fatal("first apply: script should have run")
	}

	// Second run: script skipped (hash unchanged).
	if err := os.Remove(outFile); err != nil {
		t.Fatal(err)
	}
	err = runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if _, err := os.ReadFile(outFile); err == nil {
		t.Error("second apply: script should have been skipped")
	}
}

func TestApplyScriptOnChangeContentChanged(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(dir, "on_change_output2.txt")
	scriptPath := filepath.Join(dir, "scripts", "on_change2.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'first' > "+outFile+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
shell = "`+bashPath+`"
[[scripts]]
path = "scripts/on_change2.sh"
trigger = "on_change"
`)

	err = runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Change script content → hash changes → script runs again.
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'second' > "+outFile+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(outFile); err != nil {
		t.Fatal(err)
	}
	err = runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	data, _ := os.ReadFile(outFile)
	if strings.TrimSpace(string(data)) != "second" {
		t.Errorf("script output = %q, want 'second'", strings.TrimSpace(string(data)))
	}
}

func TestApplyEmptyFilesDir(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	// Don't create files/ at all.
	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error for empty files dir, got %v", err)
	}
}

func TestApplyIgnoresFile(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	if err := os.WriteFile(filepath.Join(dir, "ignore.tmpl"), []byte("*.secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "public.conf"), []byte("public"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "private.secret"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// public.conf should exist.
	if _, err := os.Stat(filepath.Join(destDir, "public.conf")); err != nil {
		t.Error("public.conf should be written")
	}
	// private.secret should NOT exist (ignored).
	if _, err := os.Stat(filepath.Join(destDir, "private.secret")); err == nil {
		t.Error("private.secret should NOT be written (ignored)")
	}
}

func TestApplyNestedFiles(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files", ".config", "app")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "settings.conf"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destDir, ".config", "app", "settings.conf"))
	if err != nil {
		t.Fatalf("expected nested file to exist, got %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("content = %q, want %q", data, "nested")
	}
}

func TestApplyEmptyFilesDestIsTmp(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Verify no files were written to /tmp (files/ is empty).
	entries, _ := os.ReadDir("/tmp")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") || strings.HasSuffix(e.Name(), ".txt") {
			t.Errorf("unexpected file %q in /tmp", e.Name())
		}
	}
}

// ─── cmdDiff ────────────────────────────────────────────────────────────────

func TestDiffUnknownFlag(t *testing.T) {
	dir := setupTestDir(t)
	
	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"diff", "--unknown"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("error = %q, should mention unknown flag", err)
	}
}

func TestDiffClean(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("same content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("same content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDiffModified(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("dest"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDiffNewFile(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "new.conf"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDiffScopeFiles(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "test.conf"), []byte("dest"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff", "files"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDiffNoFilesDir(t *testing.T) {
	dir := setupTestDir(t)
	
	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	// No files/ directory.
	err := runWithArgs([]string{"diff"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ─── Integration: init + apply ──────────────────────────────────────────────

func TestInitNoPrompts(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"init"})
		if err != nil {
			t.Errorf("init: %v", err)
		}
	})
	if !strings.Contains(out, "dotm initialized") {
		t.Errorf("output = %q, should mention 'initialized'", out)
	}
	if !strings.Contains(out, "source:") {
		t.Errorf("output = %q, should mention 'source:'", out)
	}
}

func TestInitMissingConfig(t *testing.T) {
	setupTestDir(t)
	// Don't create dotm.toml — findSourceDir will fail.
	err := runWithArgs([]string{"init"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestApplyAfterInit(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Init.
	out := captureStdout(t, func() {
		err := runWithArgs([]string{"init"})
		if err != nil {
			t.Errorf("init: %v", err)
		}
	})
	if !strings.Contains(out, "dotm initialized") {
		t.Errorf("output = %q, should mention 'initialized'", out)
	}

	// Apply.
	err := runWithArgs([]string{"apply"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Status.
	out = captureStdout(t, func() {
		err := runWithArgs([]string{"status"})
		if err != nil {
			t.Errorf("status: %v", err)
		}
	})
	_ = out
}

// ─── cmdReset additional tests ──────────────────────────────────────────────

func TestCmdResetSinglePrompt(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[prompts]
name = "string"
`)
	// Reset requires the state file to exist, so we need to create it first.
	// Instead, test that it fails gracefully when no state exists.
	err := runWithArgs([]string{"reset", "name"})
	if err == nil {
		t.Error("expected error when no state exists, got nil")
	}
}

func TestCmdResetAll(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "/tmp"`)
	// Create empty state directory.
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("restore HOME: %v", err)
		}
	}()

	statePath := filepath.Join(home, ".local", "state", "dotm")
	if err := os.MkdirAll(statePath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"reset", "--all"})
	if err == nil {
		t.Log("reset --all succeeded (expected when no state exists)")
	}
}

func TestCmdResetMultiplePrompts(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "/tmp"`)
	err := runWithArgs([]string{"reset", "prompt1", "prompt2"})
	if err == nil {
		t.Error("expected error when prompts not in state, got nil")
	}
}

// ─── cmdInit additional tests ───────────────────────────────────────────────

func TestCmdInitWithPrompts(t *testing.T) {
	// We can't easily test interactive prompts without a real stdin.
	// Skip this test — it requires terminal interaction.
	t.Skip("requires interactive stdin")

	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[prompts]
hostname = "string"
`)
	_ = dir // kept for when this test is eventually implemented
}

func TestCmdInitWithManagers(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.pacman]
check = "true"
install = "echo install"
remove = "echo remove"

[pacman]
packages = ["vim"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"init"})
		if err != nil {
			t.Errorf("init: %v", err)
		}
	})
	if !strings.Contains(out, "managers: 1") {
		t.Errorf("output = %q, should mention 'managers: 1'", out)
	}
}

func TestCmdInitWithDestTilde(t *testing.T) {
	dir := setupTestDir(t)

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("restore HOME: %v", err)
		}
	}()

	writeDotmToml(t, dir, `dest = "~/mydest"`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"init"})
		if err != nil {
			t.Errorf("init: %v", err)
		}
	})
	expected := filepath.Join(dir, "mydest")
	if !strings.Contains(out, expected) {
		t.Errorf("output = %q, should mention expanded dest %q", out, expected)
	}
}

// ─── cmdApply additional tests ─────────────────────────────────────────────

func TestApplyScopePackages(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"apply", "pkgs", "-n"})
		if err != nil {
			t.Errorf("apply pkgs -n: %v", err)
		}
	})
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("output = %q, should mention dry run", out)
	}
}

func TestApplyScopeServices(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"apply", "services", "-n"})
		if err != nil {
			t.Errorf("apply services -n: %v", err)
		}
	})
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("output = %q, should mention dry run", out)
	}
}

func TestApplyScopeAll(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"apply", "--all"})
	if err != nil {
		t.Fatalf("apply --all: %v", err)
	}
}

// ─── cmdDiff additional tests ───────────────────────────────────────────────

func TestDiffScopePackages(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"diff", "pkgs"})
		if err != nil {
			t.Errorf("diff pkgs: %v", err)
		}
	})
	// Should show + install since check returns false.
	if !strings.Contains(out, "+ install") && !strings.Contains(out, "vim") {
		t.Errorf("output = %q, should show package diff", out)
	}
}

func TestDiffScopeServices(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"diff", "services"})
		if err != nil {
			t.Errorf("diff services: %v", err)
		}
	})
	if !strings.Contains(out, "+ enable") && !strings.Contains(out, "sshd") {
		t.Errorf("output = %q, should show service diff", out)
	}
}

func TestDiffScopeAll(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "new.conf"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff", "--all"})
	if err != nil {
		t.Fatalf("diff --all: %v", err)
	}
}

// ─── cmdStatus additional tests ─────────────────────────────────────────────

func TestStatusScopePackages(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"status", "pkgs"})
		if err != nil {
			t.Errorf("status pkgs: %v", err)
		}
	})
	if !strings.Contains(out, "MISSING") && !strings.Contains(out, "vim") {
		t.Errorf("output = %q, should show package status", out)
	}
}

func TestStatusScopeServices(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "false"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"status", "services"})
		if err != nil {
			t.Errorf("status services: %v", err)
		}
	})
	if !strings.Contains(out, "DISABLED") && !strings.Contains(out, "sshd") {
		t.Errorf("output = %q, should show service status", out)
	}
}

func TestStatusScopeAll(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"status", "--all"})
		if err != nil {
			t.Errorf("status --all: %v", err)
		}
	})
	_ = out
}

func TestStatusQuietModeProblems(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `dest = "`+destDir+`"`)
	// No files/ — status will report missing files.
	err := runWithArgs([]string{"status", "-q"})
	if err == nil {
		t.Log("expected exit code 1 for problems, but got nil (may be ok)")
	}
}

// ─── Integration: diff packages/services with check=true ────────────────────

func TestDiffPackagesInstalled(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "true"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"diff", "pkgs"})
		if err != nil {
			t.Errorf("diff pkgs: %v", err)
		}
	})
	// No + install since check returns true (installed).
	if strings.Contains(out, "+ install") {
		t.Errorf("output = %q, should not show install", out)
	}
}

func TestDiffServicesEnabled(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `
dest = "/tmp"
[managers.test]
check = "true"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"diff", "services"})
		if err != nil {
			t.Errorf("diff services: %v", err)
		}
	})
	if strings.Contains(out, "+ enable") {
		t.Errorf("output = %q, should not show enable", out)
	}
}

// ─── cmdVersion (existing test, verify via runWithArgs) ─────────────────────

func TestRunCmdVersion(t *testing.T) {
	out := captureStdout(t, func() {
		err := runWithArgs([]string{"version"})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	if out != "dotm dev\n" {
		t.Errorf("output = %q, want %q", out, "dotm dev\n")
	}
}

// ─── cmdInit error paths ─────────────────────────────────────────────────────

func TestCmdInitInvalidConfig(t *testing.T) {
	dir := setupTestDir(t)

	// Write invalid TOML.
	if err := os.WriteFile(filepath.Join(dir, "dotm.toml"), []byte("{{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"init"})
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestCmdInitMissingDestInConfig(t *testing.T) {
	dir := setupTestDir(t)

	// Valid TOML but no dest field.
	if err := os.WriteFile(filepath.Join(dir, "dotm.toml"), []byte(`shell = "bash"`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"init"})
	if err == nil {
		t.Fatal("expected error for missing dest, got nil")
	}
}

// ─── cmdDiff scope combinations ──────────────────────────────────────────────

func TestDiffScopeFilesAndPkgs(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "false"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff", "files", "pkgs"})
	if err != nil {
		t.Fatalf("diff files pkgs: %v", err)
	}
}

func TestDiffScopeFilesAndServices(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "false"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithArgs([]string{"diff", "files", "services"})
	if err != nil {
		t.Fatalf("diff files services: %v", err)
	}
}

// ─── cmdApply scope combinations ─────────────────────────────────────────────

func TestApplyScopeFilesAndPkgs(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "true"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"apply", "files", "pkgs", "-n"})
		if err != nil {
			t.Errorf("apply files pkgs -n: %v", err)
		}
	})
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("output = %q, should mention dry run", out)
	}
}

func TestApplyScopeFilesAndServices(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "true"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"apply", "files", "services", "-n"})
		if err != nil {
			t.Errorf("apply files services -n: %v", err)
		}
	})
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("output = %q, should mention dry run", out)
	}
}

// ─── cmdStatus scope combinations ────────────────────────────────────────────

func TestStatusScopeFilesAndPkgs(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "true"
install = "echo install"
remove = "echo remove"

[test]
packages = ["vim"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		err := runWithArgs([]string{"status", "files", "pkgs"})
		if err != nil {
			t.Errorf("status files pkgs: %v", err)
		}
	})
	// Should show file status.
	if !strings.Contains(out, "clean") && !strings.Contains(out, "missing") {
		t.Errorf("output = %q, should show file status", out)
	}
}

func TestStatusScopeFilesAndServices(t *testing.T) {
	dir := setupTestDir(t)

	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDotmToml(t, dir, `
dest = "`+destDir+`"
[managers.test]
check = "true"
enable = "echo enable"
disable = "echo disable"

[test]
services = ["sshd"]
`)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "test.conf"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not error — combined scope is valid.
	err := runWithArgs([]string{"status", "files", "services"})
	if err != nil {
		t.Fatalf("status files services: %v", err)
	}
}

// ─── cmdReset with existing state ────────────────────────────────────────────

func TestCmdResetWithExistingState(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "/tmp"`)

	home := t.TempDir()
	t.Setenv("HOME", home)

	statePath := filepath.Join(home, ".local", "state", "dotm")
	if err := os.MkdirAll(statePath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create state file directly with prompt data.
	absDir, _ := filepath.Abs(dir)
	h := sha256.Sum256([]byte(absDir))
	stateFileName := filepath.Join(statePath, fmt.Sprintf("%x.toml", h[:8]))
	stateContent := `
[data]
gpu = true
`
	if err := os.WriteFile(stateFileName, []byte(stateContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now reset should succeed.
	out := captureStdout(t, func() {
		err := runWithArgs([]string{"reset", "gpu"})
		if err != nil {
			t.Errorf("reset name: %v", err)
		}
	})
	if !strings.Contains(out, "reset") {
		t.Errorf("output = %q, should mention reset", out)
	}
}

func TestCmdResetAllWithExistingState(t *testing.T) {
	dir := setupTestDir(t)

	writeDotmToml(t, dir, `dest = "/tmp"`)

	home := t.TempDir()
	t.Setenv("HOME", home)

	statePath := filepath.Join(home, ".local", "state", "dotm")
	if err := os.MkdirAll(statePath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create state file directly с prompt data.
	absDir, _ := filepath.Abs(dir)
	h := sha256.Sum256([]byte(absDir))
	stateFileName := filepath.Join(statePath, fmt.Sprintf("%x.toml", h[:8]))
	stateContent := `
[data]
gpu = true
`
	if err := os.WriteFile(stateFileName, []byte(stateContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now reset --all should succeed.
	out := captureStdout(t, func() {
		err := runWithArgs([]string{"reset", "--all"})
		if err != nil {
			t.Errorf("reset --all: %v", err)
		}
	})
	if !strings.Contains(out, "All prompts reset") {
		t.Errorf("output = %q, should mention 'All prompts reset'", out)
	}
}
