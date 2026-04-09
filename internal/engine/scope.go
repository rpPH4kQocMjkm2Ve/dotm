package engine

import "fmt"

// ScopeBit is a bitmask for scopes.
type ScopeBit int

const (
	ScopeFiles   ScopeBit = 1 << iota
	ScopePkgs
	ScopeServices
	ScopeAll = ScopeFiles | ScopePkgs | ScopeServices
)

// ParseScope parses scope words from args, returning remaining flags + scope.
// Known scopes: file, files, pkg, pkgs, package, packages, service, services, all.
// Unknown tokens that start with - are treated as flags (returned as-is).
func ParseScope(args []string) (flags []string, scope ScopeBit, err error) {
	scope = 0
	for _, a := range args {
		switch a {
		case "file", "files":
			scope |= ScopeFiles
		case "pkg", "pkgs", "package", "packages":
			scope |= ScopePkgs
		case "service", "services":
			scope |= ScopeServices
		case "--all":
			scope |= ScopeAll
		default:
			if len(a) > 0 && a[0] == '-' {
				flags = append(flags, a)
			} else {
				return nil, 0, fmt.Errorf("unknown scope %q; expected file, files, pkg, pkgs, service, services, or --all", a)
			}
		}
	}
	if scope == 0 {
		scope = ScopeFiles
	}
	return flags, scope, nil
}

// Has returns true if the scope includes the given bit.
func (s ScopeBit) Has(other ScopeBit) bool {
	return s&other != 0
}
