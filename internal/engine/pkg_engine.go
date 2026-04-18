package engine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"dotm/internal/manifest"
)

// shellQuote returns a shell-safe single-quoted string.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// shellData returns template data with shell-quoted Name — used for command rendering.
func (e *Engine) shellData(name string) map[string]any {
	data := make(map[string]any, len(e.data)+1)
	data["Name"] = shellQuote(name)
	for k, v := range e.data {
		data[k] = v
	}
	return data
}

// renderCached parses and caches compiled templates to avoid redundant parsing.
func (e *Engine) renderCached(cmdTemplate string, data map[string]any) (string, error) {
	if e.tmplCache == nil {
		e.tmplCache = make(map[string]*template.Template)
	}
	tmpl, ok := e.tmplCache[cmdTemplate]
	if !ok {
		var err error
		tmpl, err = template.New("cmd").Option("missingkey=error").Parse(cmdTemplate)
		if err != nil {
			return "", fmt.Errorf("parse template: %w", err)
		}
		e.tmplCache[cmdTemplate] = tmpl
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// renderNames renders a name template and returns the result.
// If the name contains no template expressions, it is returned as-is.
// If the rendered result is empty, returns (nil, false, nil).
// Multi-line results are split into separate names.
func (e *Engine) renderNames(name string) ([]string, bool, error) {
	if !strings.Contains(name, "{{") {
		return []string{name}, name != "", nil
	}

	data := e.rawData("name")
	result, err := e.renderCached(name, data)
	if err != nil {
		return nil, false, fmt.Errorf("render %q: %w", name, err)
	}

	var names []string
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}

	if len(names) == 0 {
		return nil, false, nil
	}
	return names, true, nil
}

// rawData returns template data with raw (unquoted) values — used for name rendering.
func (e *Engine) rawData(name string) map[string]any {
	data := make(map[string]any, len(e.data)+1)
	data["Name"] = name
	for k, v := range e.data {
		data[k] = v
	}
	return data
}

func (e *Engine) check(cmdTemplate string, name string) (bool, error) {
	data := e.shellData(name)
	cmd, err := e.renderCached(cmdTemplate, data)
	if err != nil {
		return false, fmt.Errorf("render template: %w", err)
	}
	c := exec.Command("bash", "-c", cmd)
	err = c.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil // non-zero exit = not installed/enabled
		}
		return false, fmt.Errorf("check command failed to execute: %w", err)
	}
	return true, nil
}

func (e *Engine) run(cmdTemplate string, name string) error {
	data := e.shellData(name)
	cmd, err := e.renderCached(cmdTemplate, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}
	c := exec.Command("bash", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// diffPackages shows what packages would be installed or removed.
func (e *Engine) diffPackages() {
	prevPkgs, err := e.loadPkgManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: load manifest: %v\n", err)
	}

	// Check what needs installing.
	desiredPkgs := make(map[string]string) // name -> manager
	for _, pkg := range e.cfg.Packages() {
		names, ok, err := e.renderNames(pkg.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: render %s: %v\n", pkg.Name, err)
			continue
		}
		if !ok {
			continue
		}

		mgr, ok := e.cfg.Managers[pkg.Manager]
		if !ok {
			for _, name := range names {
				fmt.Printf("?        %s (manager %q not found)\n", name, pkg.Manager)
			}
			continue
		}

		for _, name := range names {
			desiredPkgs[name] = pkg.Manager
			installed, err := e.check(mgr.Check, name)
			if err != nil {
				fmt.Printf("?        %s (%s) — check error: %v\n", name, pkg.Manager, err)
				continue
			}
			if !installed {
				fmt.Printf("+ install  %s (%s)\n", name, pkg.Manager)
			}
		}
	}

	// Check what needs removing.
	for _, entry := range prevPkgs.Packages {
		if _, ok := desiredPkgs[entry.Name]; ok {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			continue
		}
		installed, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			continue
		}
		if installed {
			fmt.Printf("- remove   %s (%s)\n", entry.Name, entry.Manager)
		}
	}
}

// diffServices shows what services would be enabled or disabled.
func (e *Engine) diffServices() {
	prevPkgs, err := e.loadPkgManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: load manifest: %v\n", err)
	}
	prevSvcs := prevPkgs.Services

	// Check what needs enabling.
	desiredSvcs := make(map[string]string) // name -> manager
	for _, svc := range e.cfg.Services() {
		names, ok, err := e.renderNames(svc.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: render %s: %v\n", svc.Name, err)
			continue
		}
		if !ok {
			continue
		}

		mgr, ok := e.cfg.Managers[svc.Manager]
		if !ok {
			for _, name := range names {
				fmt.Printf("?        %s (manager %q not found)\n", name, svc.Manager)
			}
			continue
		}

		for _, name := range names {
			desiredSvcs[name] = svc.Manager
			enabled, err := e.check(mgr.Check, name)
			if err != nil {
				fmt.Printf("?        %s (%s) — check error: %v\n", name, svc.Manager, err)
				continue
			}
			if !enabled {
				fmt.Printf("+ enable   %s (%s)\n", name, svc.Manager)
			}
		}
	}

	// Check what needs disabling.
	for _, entry := range prevSvcs {
		if _, ok := desiredSvcs[entry.Name]; ok {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			continue
		}
		enabled, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			continue
		}
		if enabled {
			fmt.Printf("- disable  %s (%s)\n", entry.Name, entry.Manager)
		}
	}
}

// applyPackages installs desired packages and removes obsolete ones.
// Returns the updated package manifest entries and any errors.
func (e *Engine) applyPackages(dryRun bool) ([]manifest.PackageEntry, []error) {
	prevPkgs, err := e.loadPkgManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: load manifest: %v\n", err)
	}

	desiredPkgs := make(map[string]string) // name -> manager
	var pkgEntries []manifest.PackageEntry
	var errs []error

	for _, pkg := range e.cfg.Packages() {
		names, ok, err := e.renderNames(pkg.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: render %s: %v\n", pkg.Name, err)
			continue
		}
		if !ok {
			continue
		}

		mgr := e.cfg.Managers[pkg.Manager]

		for _, name := range names {
			desiredPkgs[name] = pkg.Manager

			if dryRun {
				fmt.Printf("[DRY RUN] Would check and potentially install: %s (%s)\n", name, pkg.Manager)
				continue
			}

			installed, err := e.check(mgr.Check, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: check %s: %v\n", name, err)
				continue
			}

			if !installed {
				if err := e.run(mgr.Install, name); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: install %s: %v\n", name, err)
					errs = append(errs, fmt.Errorf("install %s (%s): %w", name, pkg.Manager, err))
					continue
				}
				fmt.Printf("Installed: %s (%s)\n", name, pkg.Manager)
			}

			pkgEntries = append(pkgEntries, manifest.PackageEntry{
				Name:    name,
				Manager: pkg.Manager,
			})
		}
	}

	// Remove obsolete packages.
	for _, entry := range prevPkgs.Packages {
		if _, ok := desiredPkgs[entry.Name]; ok {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			fmt.Fprintf(os.Stderr, "WARN: manager %q for %s not found, skipping\n", entry.Manager, entry.Name)
			continue
		}

		if dryRun {
			fmt.Printf("[DRY RUN] Would check and potentially remove: %s (%s)\n", entry.Name, entry.Manager)
			continue
		}

		installed, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: check %s: %v\n", entry.Name, err)
			continue
		}
		if installed {
			if err := e.run(mgr.Remove, entry.Name); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: remove %s: %v\n", entry.Name, err)
				errs = append(errs, fmt.Errorf("remove %s (%s): %w", entry.Name, entry.Manager, err))
				continue
			}
			fmt.Printf("Removed: %s (%s)\n", entry.Name, entry.Manager)
		}
	}

	return pkgEntries, errs
}

// applyServices enables desired services and disables obsolete ones.
// Returns the updated service manifest entries and any errors.
func (e *Engine) applyServices(dryRun bool) ([]manifest.ServiceEntry, []error) {
	prevPkgs, err := e.loadPkgManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: load manifest: %v\n", err)
	}
	prevSvcs := prevPkgs.Services

	desiredSvcs := make(map[string]string) // name -> manager
	var svcEntries []manifest.ServiceEntry
	var errs []error

	for _, svc := range e.cfg.Services() {
		names, ok, err := e.renderNames(svc.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: render %s: %v\n", svc.Name, err)
			continue
		}
		if !ok {
			continue
		}

		mgr := e.cfg.Managers[svc.Manager]

		for _, name := range names {
			desiredSvcs[name] = svc.Manager

			if dryRun {
				fmt.Printf("[DRY RUN] Would check and potentially enable: %s (%s)\n", name, svc.Manager)
				continue
			}

			enabled, err := e.check(mgr.Check, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: check service %s: %v\n", name, err)
				continue
			}

			if !enabled {
				if err := e.run(mgr.Enable, name); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: enable %s: %v\n", name, err)
					errs = append(errs, fmt.Errorf("enable %s (%s): %w", name, svc.Manager, err))
					continue
				}
				fmt.Printf("Enabled: %s (%s)\n", name, svc.Manager)
			}

			svcEntries = append(svcEntries, manifest.ServiceEntry{
				Name:    name,
				Manager: svc.Manager,
			})
		}
	}

	// Disable obsolete services.
	for _, entry := range prevSvcs {
		if _, ok := desiredSvcs[entry.Name]; ok {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			fmt.Fprintf(os.Stderr, "WARN: manager %q for service %s not found, skipping\n", entry.Manager, entry.Name)
			continue
		}

		if dryRun {
			fmt.Printf("[DRY RUN] Would check and potentially disable: %s (%s)\n", entry.Name, entry.Manager)
			continue
		}

		enabled, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: check service %s: %v\n", entry.Name, err)
			continue
		}
		if enabled {
			if err := e.run(mgr.Disable, entry.Name); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: disable %s: %v\n", entry.Name, err)
				errs = append(errs, fmt.Errorf("disable %s (%s): %w", entry.Name, entry.Manager, err))
				continue
			}
			fmt.Printf("Disabled: %s (%s)\n", entry.Name, entry.Manager)
		}
	}

	return svcEntries, errs
}

func (e *Engine) loadPkgManifest() (*manifest.PkgManifest, error) {
	m, err := manifest.Load(e.configDir)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// savePkgManifest writes the combined package/service manifest to state.
func (e *Engine) savePkgManifest(pkgs []manifest.PackageEntry, svcs []manifest.ServiceEntry) error {
	m := &manifest.PkgManifest{
		Packages: pkgs,
		Services: svcs,
	}
	return manifest.Save(e.configDir, m)
}
