package engine

import (
	"dotm/internal/manifest"
	"fmt"
	"os"
)

// statusPackages prints the status of managed packages.
func (e *Engine) statusPackages(report *StatusReport, prevPkgs *manifest.PkgManifest, verbose bool) error {
	if len(e.cfg.Managers) == 0 {
		return nil
	}

	desiredPkgs := make(map[string]bool)
	headerPrinted := false

	for _, pkg := range e.cfg.Packages() {
		name, ok, err := e.renderName(pkg.Name)
		if err != nil {
			fmt.Printf("  ?        %s (%s) — %v\n", pkg.Name, pkg.Manager, err)
			report.PkgHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		if !ok {
			if verbose {
				if !headerPrinted {
					fmt.Println("Packages:")
					headerPrinted = true
					report.PkgOrSvcPrinted = true
				}
				fmt.Printf("  SKIP     %s (%s) — template rendered empty\n", pkg.Name, pkg.Manager)
			}
			continue
		}
		desiredPkgs[name] = true

		mgr, ok := e.cfg.Managers[pkg.Manager]
		if !ok {
			fmt.Printf("  ?        %s (manager %q not found)\n", name, pkg.Manager)
			report.PkgHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		installed, err := e.check(mgr.Check, name)
		if err != nil {
			fmt.Printf("  ?        %s (%s) — check error: %v\n", name, pkg.Manager, err)
			report.PkgHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		if installed {
			if verbose {
				if !headerPrinted {
					fmt.Println("Packages:")
					headerPrinted = true
					report.PkgOrSvcPrinted = true
				}
				fmt.Printf("  OK       %s (%s)\n", name, pkg.Manager)
			}
		} else {
			if !headerPrinted {
				fmt.Println("Packages:")
				headerPrinted = true
				report.PkgOrSvcPrinted = true
			}
			fmt.Printf("  MISSING  %s (%s)\n", name, pkg.Manager)
			report.PkgHasProblems = true
		}
	}

	// Obsolete packages.
	for _, entry := range prevPkgs.Packages {
		if desiredPkgs[entry.Name] {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			fmt.Fprintf(os.Stderr, "WARN: manager %q for %s not found, skipping\n", entry.Manager, entry.Name)
			continue
		}
		installed, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: check %s: %v\n", entry.Name, err)
			continue
		}
		if installed {
			if !headerPrinted {
				fmt.Println("Packages:")
				headerPrinted = true
				report.PkgOrSvcPrinted = true
			}
			fmt.Printf("  OBSOLETE %s (%s) — still installed\n", entry.Name, entry.Manager)
			report.PkgHasProblems = true
		}
	}

	return nil
}

// statusServices prints the status of managed services.
func (e *Engine) statusServices(report *StatusReport, prevPkgs *manifest.PkgManifest, verbose bool) error {
	if len(e.cfg.Managers) == 0 {
		return nil
	}

	prevSvcs := prevPkgs.Services
	desiredSvcs := make(map[string]bool)
	headerPrinted := false

	for _, svc := range e.cfg.Services() {
		name, ok, err := e.renderName(svc.Name)
		if err != nil {
			fmt.Printf("  ?        %s (%s) — %v\n", svc.Name, svc.Manager, err)
			report.SvcHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		if !ok {
			if verbose {
				if !headerPrinted {
					fmt.Println("Services:")
					headerPrinted = true
					report.PkgOrSvcPrinted = true
				}
				fmt.Printf("  SKIP     %s (%s) — template rendered empty\n", svc.Name, svc.Manager)
			}
			continue
		}
		desiredSvcs[name] = true

		mgr, ok := e.cfg.Managers[svc.Manager]
		if !ok {
			fmt.Printf("  ?        %s (manager %q not found)\n", name, svc.Manager)
			report.SvcHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		enabled, err := e.check(mgr.Check, name)
		if err != nil {
			fmt.Printf("  ?        %s (%s) — check error: %v\n", name, svc.Manager, err)
			report.SvcHasProblems = true
			report.PkgOrSvcPrinted = true
			continue
		}
		if enabled {
			if verbose {
				if !headerPrinted {
					if report.PkgOrSvcPrinted {
						fmt.Println("\nServices:")
					} else {
						fmt.Println("Services:")
					}
					headerPrinted = true
					report.PkgOrSvcPrinted = true
				}
				fmt.Printf("  ENABLED  %s (%s)\n", name, svc.Manager)
			}
		} else {
			if !headerPrinted {
				if report.PkgOrSvcPrinted {
					fmt.Println("\nServices:")
				} else {
					fmt.Println("Services:")
				}
				headerPrinted = true
				report.PkgOrSvcPrinted = true
			}
			fmt.Printf("  DISABLED %s (%s)\n", name, svc.Manager)
			report.SvcHasProblems = true
		}
	}

	// Obsolete services.
	for _, entry := range prevSvcs {
		if desiredSvcs[entry.Name] {
			continue
		}
		mgr, ok := e.cfg.Managers[entry.Manager]
		if !ok {
			fmt.Fprintf(os.Stderr, "WARN: manager %q for service %s not found, skipping\n", entry.Manager, entry.Name)
			continue
		}
		enabled, err := e.check(mgr.Check, entry.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: check service %s: %v\n", entry.Name, err)
			continue
		}
		if enabled {
			if !headerPrinted {
				if report.PkgOrSvcPrinted {
					fmt.Println("\nServices:")
				} else {
					fmt.Println("Services:")
				}
				headerPrinted = true
				report.PkgOrSvcPrinted = true
			}
			fmt.Printf("  OBSOLETE %s (%s) — still enabled\n", entry.Name, entry.Manager)
			report.SvcHasProblems = true
		}
	}

	return nil
}

