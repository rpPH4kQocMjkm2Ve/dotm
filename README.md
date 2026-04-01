# dotm

[![CI](https://github.com/rpPH4kQocMjkm2Ve/dotm/actions/workflows/ci.yml/badge.svg)](https://github.com/rpPH4kQocMjkm2Ve/dotm/actions/workflows/ci.yml)
![License](https://img.shields.io/github/license/rpPH4kQocMjkm2Ve/dotm)

Declarative dotfiles manager. Lightweight chezmoi alternative with normal file paths, delegated encryption, proper permission management, and first-class support for `dest = "/"`.

## How it works

```
dotm apply
      ↓
  1. Walk files/ directory
  2. Render .tmpl templates with Go template engine
  3. Write to dest (skip unchanged files)
  4. Create symlinks from [symlinks] map
  5. Apply permission rules from perms file
  6. Run lifecycle scripts
  7. Record manifest for orphan tracking
```

The repo is the single source of truth. `apply` is a one-directional push. No bidirectional sync, no source state attributes, no magic prefixes.

## Why not chezmoi

| | dotm | chezmoi |
|---|---|---|
| **File naming** | `.config/hypr/hyprland.conf` | `dot_config/private_dot_hypr/hyprland.conf` |
| **Encryption** | Delegate to sops, age, etc. | Built-in age/gpg |
| **Permissions** | First-class `perms` file with glob patterns | Limited (encoded in filename prefixes) |
| **`dest = "/"`** | Works out of the box | Needs workarounds |
| **Complexity** | ~2k LOC | ~50k LOC |

## Installation

### From source

```bash
git clone https://gitlab.com/fkzys/dotm.git
cd dotm
make install        # builds and installs to /usr/bin/dotm
```

### Build only

```bash
make build          # produces ./dotm binary
# or directly:
go build -o dotm ./cmd/dotm/
```

Requires Go 1.24+.

## Repository layout

```
~/dotfiles/
├── dotm.toml           # config (required)
├── files/              # your actual dotfiles (mirrors dest)
│   ├── .config/
│   │   ├── hypr/
│   │   │   └── hyprland.conf
│   │   └── waybar/
│   │       ├── config.jsonc
│   │       └── style.css.tmpl
│   ├── .zshrc.tmpl
│   └── etc/            # system files when dest = "/"
│       └── pacman.conf
├── perms               # permission rules (optional)
├── ignore.tmpl         # ignore patterns (optional)
└── scripts/            # lifecycle scripts (optional)
    └── reload.sh
```

Files under `files/` are copied to `dest` preserving directory structure. Files ending in `.tmpl` are rendered as Go templates, with the suffix stripped in the output.

## Configuration

`dotm.toml`:

```toml
dest = "~"

# Interactive prompts — values available in templates as {{ .use_nvidia }}
[prompts]
use_nvidia = { type = "bool", question = "Enable NVIDIA config?" }
git_email  = { type = "string", question = "Git email address" }

# Symlinks: link (relative to dest) → target
[symlinks]
".local/bin/editor" = "{{ .homeDir }}/.nix-profile/bin/nvim"

# Lifecycle scripts
[[scripts]]
path = "scripts/reload.sh"
template = true         # render as template before running
trigger = "on_change"   # "always" or "on_change"
```

For system-wide configuration:

```toml
dest = "/"
```

Files in `files/etc/pacman.conf` deploy to `/etc/pacman.conf`.

## Usage

```bash
dotm init              # resolve prompts, create state cache
dotm apply             # deploy everything
dotm apply -n          # dry run — show what would happen
dotm diff              # unified diff between source and dest
dotm status            # show sync state of all managed files
dotm status -v         # include clean files
dotm status -q         # exit 1 if problems exist, no output
dotm help              # show help
```

### Example apply

```
$ dotm apply
mkdir /home/user/.config/hypr
write /home/user/.config/hypr/hyprland.conf (2847 bytes)
write /home/user/.zshrc (1204 bytes)
symlink /home/user/.local/bin/editor -> /home/user/.nix-profile/bin/nvim
run scripts/reload.sh
```

### Example status

```
$ dotm status
  modified   .config/waybar/style.css
  missing    .config/foot/foot.ini
  orphan     .config/sway/config
```

Four states:
- **clean** — dest matches rendered source
- **modified** — dest differs from source
- **missing** — in source but not yet in dest
- **orphan** — was deployed previously, no longer in source, still in dest

dotm never auto-deletes orphans. It reports them; you decide.

## Templates

Files ending in `.tmpl` are rendered with Go's `text/template`.

### Built-in variables

| Variable | Value |
|----------|-------|
| `{{ .homeDir }}` | User home directory |
| `{{ .hostname }}` | System hostname |
| `{{ .username }}` | Current user |
| `{{ .sourceDir }}` | Absolute path to dotfiles repo |

Prompt values are available by name: `{{ .use_nvidia }}`, `{{ .git_email }}`.

### Custom functions

| Function | Description |
|----------|-------------|
| `output "cmd" "arg1" "arg2"` | Run command, return stdout |
| `fromYaml` | Parse YAML string into map |
| `joinPath "a" "b"` | `filepath.Join` |
| `hasKey $map "key"` | Check if map contains key |
| `replace "old" "new" $s` | Replace all occurrences |

### Example: secrets via sops

`files/.config/app/config.yaml.tmpl`:

```yaml
{{ $s := output "sops" "-d" (joinPath .sourceDir "secrets.enc.yaml") | fromYaml -}}
api_key: {{ index $s "api_key" }}
db_password: {{ index $s "db_password" }}
```

No built-in encryption. sops/age/gpg handle decryption; dotm handles templating and deployment.

### Example: conditional blocks

`files/.zshrc.tmpl`:

```bash
export EDITOR="{{ .editor }}"

{{ if .use_nvidia -}}
export __GL_SHADER_DISK_CACHE_PATH="{{ .homeDir }}/.cache/nv"
export __GL_SHADER_DISK_CACHE_SIZE=1073741824
{{ end -}}
```

## Ignore patterns

`ignore.tmpl` (rendered as template, then parsed as glob patterns):

```
# Always ignore
.git/**
*.swp
.DS_Store

# Conditional
{{ if not .use_nvidia -}}
.config/nvidia/**
{{ end -}}
```

Patterns are matched against paths relative to `files/`. Supports `*`, `?`, `**`.

## Permission management

The `perms` file sets mode, owner, and group on deployed files:

```bash
# pattern              mode   owner  group
# Trailing / = directories only, no / = files only
# - = don't change that attribute
# Last matching rule wins

etc/**/                0755   root   root
etc/**                 0644   root   root

etc/security/          0700   root   root
etc/security/**        0600   root   root

etc/polkit-1/rules.d/**  0640  root  polkitd

root/**/               0700   root   root
root/**                0600   root   root
```

Glob patterns support `*`, `?`, `**`. Rules are evaluated top-to-bottom; last match wins. Directory rules (trailing `/`) only match directories; file rules only match files.

This is the primary reason dotm exists as a separate tool — managing `/etc` permissions correctly matters, and encoding `0640 root:polkitd` in a filename is not a serious approach.

## Scripts

```toml
[[scripts]]
path = "scripts/reload-hypr.sh"
template = false
trigger = "always"        # run on every apply

[[scripts]]
path = "scripts/setup.sh.tmpl"
template = true           # render before running
trigger = "on_change"     # run only when content changes
```

Scripts are executed with `bash`. `on_change` tracks content hash in state — if the rendered script hasn't changed since last apply, it's skipped.

## State

dotm stores state in `~/.local/state/dotm/<hash>.toml`:

- **Prompt answers** — cached so you're not asked every apply
- **Script hashes** — for `on_change` trigger
- **Manifest** — list of deployed files for orphan detection

Each source repo gets its own state file (keyed by SHA-256 of absolute path). Re-run `dotm init` to re-answer prompts.

## Tests

```bash
make test               # run all tests
make test-root          # run permission tests (requires root)
```

Permission tests (`internal/perms/`) need root for `chmod`/`chown` verification.

## Dependencies

Runtime: none (static Go binary).

Build: Go 1.24+.

Go module dependencies:
- `github.com/BurntSushi/toml` — TOML parsing
- `gopkg.in/yaml.v3` — `fromYaml` template function

External tools (optional, used by templates at runtime):
- `sops` — if your templates call `output "sops" ...`
- `diff` — used by `dotm diff` (present on any unix system)
- `bash` — script execution

## License

AGPL-3.0-or-later
