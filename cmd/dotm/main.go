package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dotm/internal/config"
	"dotm/internal/engine"
	"dotm/internal/prompt"
)

const configName = "dotm.toml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "dotm: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]

	if len(args) == 0 {
		return usageError()
	}

	cmd := args[0]
	flags := args[1:]

	switch cmd {
	case "init":
		return cmdInit()
	case "apply":
		return cmdApply(flags)
	case "diff":
		return cmdDiff()
	case "status":
		return cmdStatus(flags)
	case "help", "--help", "-h":
		printUsage()
		return nil
	case "version", "--version", "-V":
		cmdVersion()
		return nil
	default:
		return fmt.Errorf("unknown command %q\nrun 'dotm help' for usage", cmd)
	}
}

func cmdInit() error {
	sourceDir, err := findSourceDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(sourceDir, configName))
	if err != nil {
		return err
	}

	state, err := prompt.LoadState(sourceDir)
	if err != nil {
		return err
	}

	changed, err := prompt.Resolve(cfg, state, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	if changed {
		if err := state.Save(sourceDir); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	// Show summary.
	fmt.Printf("\ndotm initialized\n")
	fmt.Printf("  source: %s\n", sourceDir)
	fmt.Printf("  dest:   %s\n", cfg.Dest)
	fmt.Printf("  state:  %s\n", prompt.FormatStateFile(sourceDir))

	if len(state.Data) > 0 {
		fmt.Printf("\n  data:\n")
		for k, v := range state.Data {
			fmt.Printf("    %s = %s\n", k, prompt.FormatPromptValue(v))
		}
	}

	return nil
}

func cmdApply(flags []string) error {
	dryRun := false
	for _, f := range flags {
		switch f {
		case "-n", "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown flag %q for apply", f)
		}
	}

	sourceDir, err := findSourceDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(sourceDir, configName))
	if err != nil {
		return err
	}

	state, err := prompt.LoadState(sourceDir)
	if err != nil {
		return err
	}

	// Resolve any new prompts (e.g. added since last init).
	changed, err := prompt.Resolve(cfg, state, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	eng, err := engine.New(cfg, state, sourceDir, dryRun)
	if err != nil {
		return err
	}

	if err := eng.Apply(); err != nil {
		return err
	}

	// Save state (prompt answers + script hashes + manifest).
	if changed || !dryRun {
		if err := state.Save(sourceDir); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	return nil
}

func cmdDiff() error {
	sourceDir, err := findSourceDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(sourceDir, configName))
	if err != nil {
		return err
	}

	state, err := prompt.LoadState(sourceDir)
	if err != nil {
		return err
	}

	// Resolve prompts if needed (diff may render templates).
	if _, err := prompt.Resolve(cfg, state, os.Stdin, os.Stdout); err != nil {
		return err
	}

	eng, err := engine.New(cfg, state, sourceDir, false)
	if err != nil {
		return err
	}

	return eng.Diff()
}

func cmdStatus(flags []string) error {
	verbose := false
	quiet := false
	for _, f := range flags {
		switch f {
		case "-v", "--verbose":
			verbose = true
		case "-q", "--quiet":
			quiet = true
		default:
			return fmt.Errorf("unknown flag %q for status", f)
		}
	}

	sourceDir, err := findSourceDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(sourceDir, configName))
	if err != nil {
		return err
	}

	state, err := prompt.LoadState(sourceDir)
	if err != nil {
		return err
	}

	// Resolve prompts if needed (status may render templates).
	if _, err := prompt.Resolve(cfg, state, os.Stdin, os.Stdout); err != nil {
		return err
	}

	eng, err := engine.New(cfg, state, sourceDir, false)
	if err != nil {
		return err
	}

	report, err := eng.Status()
	if err != nil {
		return err
	}

	if quiet {
		if report.HasProblems() {
			os.Exit(1)
		}
		return nil
	}

	engine.PrintReport(report, verbose)

	if !verbose && !report.HasProblems() {
		fmt.Println("  all clean")
	}

	return nil
}

// findSourceDir walks up from the current directory looking for dotm.toml.
func findSourceDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, configName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("%s not found in current directory or any parent", configName)
}

func usageError() error {
	return fmt.Errorf("no command specified\nrun 'dotm help' for usage")
}

func printUsage() {
	fmt.Print(`dotm — dotfiles manager

Usage:
  dotm <command> [flags]

Commands:
  init       Run prompts interactively and create state cache
  apply      Apply files, symlinks, perms, and scripts to dest
  apply -n   Dry run — show what would be done without writing
  diff       Show unified diff between source and dest
  status     Show sync state of managed files
  status -v  Show all files including clean ones
  status -q  Exit 1 if any problems, no output
  version    Print version and exit
  help       Show this help

dotm looks for dotm.toml in the current directory or any parent.
`)
}
