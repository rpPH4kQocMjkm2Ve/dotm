package engine

import (
	"testing"
)

func TestParseScope(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantFlags   []string
		wantScope   ScopeBit
		wantErr     bool
	}{
		{
			name:      "empty args",
			args:      []string{},
			wantScope: ScopeFiles, // default
		},
		{
			name:      "file",
			args:      []string{"file"},
			wantScope: ScopeFiles,
		},
		{
			name:      "files",
			args:      []string{"files"},
			wantScope: ScopeFiles,
		},
		{
			name:      "pkg",
			args:      []string{"pkg"},
			wantScope: ScopePkgs,
		},
		{
			name:      "pkgs",
			args:      []string{"pkgs"},
			wantScope: ScopePkgs,
		},
		{
			name:      "package",
			args:      []string{"package"},
			wantScope: ScopePkgs,
		},
		{
			name:      "packages",
			args:      []string{"packages"},
			wantScope: ScopePkgs,
		},
		{
			name:      "service",
			args:      []string{"service"},
			wantScope: ScopeServices,
		},
		{
			name:      "services",
			args:      []string{"services"},
			wantScope: ScopeServices,
		},
		{
			name:      "--all",
			args:      []string{"--all"},
			wantScope: ScopeAll,
		},
		{
			name:      "combined files + pkg",
			args:      []string{"files", "pkg"},
			wantScope: ScopeFiles | ScopePkgs,
		},
		{
			name:      "combined all + services",
			args:      []string{"--all", "services"},
			wantScope: ScopeAll | ScopeServices, // all already includes services
		},
		{
			name:      "flags preserved",
			args:      []string{"--verbose", "--dry-run", "pkg"},
			wantFlags: []string{"--verbose", "--dry-run"},
			wantScope: ScopePkgs,
		},
		{
			name:      "unknown scope error",
			args:      []string{"unknown"},
			wantErr:   true,
		},
		{
			name:      "mixed flags and scopes",
			args:      []string{"--dry-run", "files", "--verbose", "services"},
			wantFlags: []string{"--dry-run", "--verbose"},
			wantScope: ScopeFiles | ScopeServices,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, scope, err := ParseScope(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScope() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if scope != tt.wantScope {
				t.Errorf("ParseScope() scope = %v, want %v", scope, tt.wantScope)
			}
			if len(flags) != len(tt.wantFlags) {
				t.Errorf("ParseScope() flags = %v, want %v", flags, tt.wantFlags)
				return
			}
			for i, f := range flags {
				if f != tt.wantFlags[i] {
					t.Errorf("ParseScope() flags[%d] = %q, want %q", i, f, tt.wantFlags[i])
				}
			}
		})
	}
}

func TestScopeBit_Has(t *testing.T) {
	tests := []struct {
		name  string
		scope ScopeBit
		other ScopeBit
		want  bool
	}{
		{"files has files", ScopeFiles, ScopeFiles, true},
		{"files has pkgs", ScopeFiles, ScopePkgs, false},
		{"all has files", ScopeAll, ScopeFiles, true},
		{"all has pkgs", ScopeAll, ScopePkgs, true},
		{"all has services", ScopeAll, ScopeServices, true},
		{"files|pkgs has services", ScopeFiles | ScopePkgs, ScopeServices, false},
		{"files|pkgs has pkgs", ScopeFiles | ScopePkgs, ScopePkgs, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.Has(tt.other); got != tt.want {
				t.Errorf("ScopeBit(%v).Has(%v) = %v, want %v", tt.scope, tt.other, got, tt.want)
			}
		})
	}
}
