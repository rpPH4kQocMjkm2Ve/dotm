package engine

import (
	"testing"

	"dotm/internal/config"
)

func newTestPkgEngine(data map[string]any) *Engine {
	return &Engine{
		cfg:  &config.Config{Managers: map[string]config.ManagerConfig{"test": {}}},
		data: data,
	}
}

func TestRenderName(t *testing.T) {
	data := map[string]any{"laptop": true, "tablet": false}
	e := &Engine{data: data}

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"plain", "git", "git", true},
		{"conditional true", "{{ if .laptop }}laptop-tools{{ end }}", "laptop-tools", true},
		{"conditional false", "{{ if .tablet }}tablet-driver{{ end }}", "", false},
		{"parse error", "{{ end }}", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := e.renderName(tt.input)
			if ok != tt.ok {
				t.Errorf("renderName(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if err != nil && tt.name != "parse error" {
				t.Errorf("renderName(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("renderName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	data := map[string]any{"Name": "foo", "laptop": true}

	tests := []struct {
		name     string
		template string
		want     string
		wantErr  bool
	}{
		{
			name:     "no template",
			template: "pacman -Q foo",
			want:     "pacman -Q foo",
			wantErr:  false,
		},
		{
			name:     "with Name variable",
			template: "pacman -Q {{.Name}}",
			want:     "pacman -Q foo",
			wantErr:  false,
		},
		{
			name:     "multiple substitutions",
			template: "systemctl enable {{.Name}} && systemctl start {{.Name}}",
			want:     "systemctl enable foo && systemctl start foo",
			wantErr:  false,
		},
		{
			name:     "missing key error",
			template: "cmd {{.Unknown}}",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "syntax error",
			template: "cmd {{.Name",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTestPkgEngine(data).renderCached(tt.template, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}
