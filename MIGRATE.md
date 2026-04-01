# Migrating from chezmoi to dotm

## Overview

dotm uses normal file paths instead of chezmoi's encoded prefixes (`dot_`, `executable_`, `private_`, `symlink_`). Templates use the same Go `text/template` engine with a slightly different variable namespace. Permissions are managed via a dedicated `perms` file instead of filename prefixes. Symlinks and scripts are declared in `dotm.toml`.

## Conceptual mapping

| Concept | chezmoi | dotm |
|---|---|---|
| Source directory | `~/.local/share/chezmoi/` | Any directory with `dotm.toml` |
| File prefix `dot_` | `dot_config/` → `.config/` | `.config/` (literal path) |
| File prefix `executable_` | Sets 0755 on file | `perms` file rule |
| File prefix `private_` | Sets 0700 on dir / 0600 on file | `perms` file rule |
| File prefix `symlink_` | Creates symlink from template content | `[symlinks]` in `dotm.toml` |
| Config file | `~/.config/chezmoi/chezmoi.toml` | `dotm.toml` in repo root |
| Prompts | `promptBoolOnce` / `promptStringOnce` in `.chezmoi.toml.tmpl` | `[prompts]` table in `dotm.toml` |
| Ignore patterns | `.chezmoiignore` (template) | `ignore.tmpl` (template) |
| Encryption | Built-in age/gpg (`encryption = "age"`) | Delegated — use `output "sops" ...` in templates |
| Run scripts | `run_onchange_*.sh.tmpl` in source root | `[[scripts]]` in `dotm.toml` |
| State | `~/.config/chezmoi/chezmoistate.boltdb` | `~/.local/state/dotm/<hash>.toml` |
| Files directory | Source root (mixed with config) | `files/` subdirectory |

## Template variable changes

All chezmoi built-in variables drop the `.chezmoi.` prefix:

| chezmoi | dotm |
|---|---|
| `{{ .chezmoi.homeDir }}` | `{{ .homeDir }}` |
| `{{ .chezmoi.hostname }}` | `{{ .hostname }}` |
| `{{ .chezmoi.username }}` | `{{ .username }}` |
| `{{ .chezmoi.sourceDir }}` | `{{ .sourceDir }}` |

User-defined data variables (from prompts) remain unchanged:

```
{{ .nvidia }}        — same in both
{{ .laptop }}        — same in both
{{ eq .ocr true }}   — same in both
```

## Template functions

dotm provides these custom functions (all compatible with chezmoi usage):

| Function | Signature | Notes |
|---|---|---|
| `output` | `output "cmd" "arg1" "arg2"` | Same as chezmoi |
| `fromYaml` | `output ... \| fromYaml` | Same as chezmoi |
| `joinPath` | `joinPath "a" "b"` | Same as chezmoi |
| `hasKey` | `hasKey $map "key"` | Same as chezmoi |
| `replace` | `replace "old" "new" $s` | Same argument order as chezmoi |

Standard Go template functions (`index`, `eq`, `if`, `range`, `printf`, etc.) work identically.

## Step-by-step migration

### 1. Create the new repository structure

```bash
NEW="$HOME/dotfiles"
mkdir -p "$NEW/files" "$NEW/scripts"
```

### 2. Convert file names

chezmoi encodes metadata in filenames. dotm uses real paths under `files/`. Strip all prefixes:

| chezmoi prefix | Action |
|---|---|
| `dot_` | Replace with `.` |
| `executable_` | Remove (handle in `perms`) |
| `private_` | Remove (handle in `perms`) |
| `literal_` | Remove |
| `symlink_` | Don't copy — add to `[symlinks]` in `dotm.toml` |

Example transformations:

```
dot_config/hypr/hyprland.conf.tmpl
  → files/.config/hypr/hyprland.conf.tmpl

dot_config/hypr/scripts/executable_lock.sh
  → files/.config/hypr/scripts/lock.sh

private_dot_ssh/config
  → files/.ssh/config

dot_config/private_fcitx5/private_conf/private_classicui.conf
  → files/.config/fcitx5/conf/classicui.conf

dot_local/bin/executable_anki.tmpl
  → files/.local/bin/anki.tmpl

dot_local/bin/symlink_imv-dir.tmpl
  → [symlinks] entry in dotm.toml (not a file)

executable_literal_run_sing-box.sh.tmpl
  → files/run_sing-box.sh.tmpl
```

Automated conversion script:

```bash
OLD="$HOME/.local/share/chezmoi"
NEW="$HOME/dotfiles"

process_dir() {
    local src="$1" dst="$2"
    for item in "$src"/*; do
        [[ ! -e "$item" ]] && continue
        local name newname
        name=$(basename "$item")
        newname="$name"

        # Skip chezmoi metadata and repo files
        case "$name" in
            run_onchange_*|.chezmoi*|.chezmoiignore|.sops.yaml|\
            secrets.enc.yaml|LICENSE|README.md|assets|.envrc)
                continue ;;
        esac

        # Strip prefixes (order matters)
        newname="${newname#private_}"
        newname="${newname#executable_}"
        newname="${newname#literal_}"
        [[ "$newname" == dot_* ]] && newname=".${newname#dot_}"

        # symlink_ files go to dotm.toml, not files/
        [[ "$name" == symlink_* ]] && continue

        if [[ -d "$item" ]]; then
            mkdir -p "$dst/$newname"
            process_dir "$item" "$dst/$newname"
        else
            cp "$item" "$dst/$newname"
        fi
    done
}
process_dir "$OLD" "$NEW/files"
```

### 3. Fix template variables

Replace all chezmoi-namespaced variables:

```bash
find "$NEW" -name '*.tmpl' -exec sed -i \
    -e 's/\.chezmoi\.homeDir/.homeDir/g' \
    -e 's/\.chezmoi\.hostname/.hostname/g' \
    -e 's/\.chezmoi\.sourceDir/.sourceDir/g' \
    -e 's/\.chezmoi\.username/.username/g' \
    {} +
```

No other template changes are needed. All `output`, `fromYaml`, `joinPath`, `hasKey`, `replace`, `index`, `eq` calls work identically.

### 4. Convert symlink_ files to [symlinks]

In chezmoi, `symlink_` files contain the symlink target as template content. In dotm, symlinks are declared in `dotm.toml`:

**chezmoi** — `dot_local/bin/symlink_imv-dir.tmpl`:
```
/home/{{ .chezmoi.username }}/.local/bin/imv
```

**dotm** — `dotm.toml`:
```toml
[symlinks]
".local/bin/imv-dir" = "{{ .homeDir }}/.local/bin/imv"
```

The key is the link path (relative to `dest`), the value is the target (supports template expressions).

### 5. Convert run scripts

chezmoi uses specially-named files in the source root:

```
run_onchange_install_packages.sh.tmpl
run_onchange_after_enable_services.sh.tmpl
```

dotm uses `[[scripts]]` entries in `dotm.toml` with explicit trigger configuration:

```bash
# Copy scripts to scripts/ directory
cp "$OLD/run_onchange_install_packages.sh.tmpl" "$NEW/scripts/install_packages.sh.tmpl"
cp "$OLD/run_onchange_after_enable_services.sh.tmpl" "$NEW/scripts/enable_services.sh.tmpl"
```

```toml
# dotm.toml
[[scripts]]
path = "scripts/install_packages.sh.tmpl"
template = true
trigger = "on_change"    # equivalent to chezmoi's run_onchange_

[[scripts]]
path = "scripts/enable_services.sh.tmpl"
template = true
trigger = "on_change"
```

| chezmoi naming convention | dotm equivalent |
|---|---|
| `run_` prefix | `trigger = "always"` |
| `run_onchange_` prefix | `trigger = "on_change"` |
| `.tmpl` suffix | `template = true` |

### 6. Convert prompts

**chezmoi** — `.chezmoi.toml.tmpl`:
```
{{- $nvidia := promptBoolOnce . "nvidia" "NVIDIA GPU" -}}
{{- $laptop := promptBoolOnce . "laptop" "Laptop" -}}

[data]
    nvidia = {{ $nvidia }}
    laptop = {{ $laptop }}
```

**dotm** — `dotm.toml`:
```toml
[prompts]
nvidia = { type = "bool", question = "NVIDIA GPU" }
laptop = { type = "bool", question = "Laptop (battery, backlight, compact fonts)" }
```

Prompt values are cached in `~/.local/state/dotm/<hash>.toml` and available in templates as `{{ .nvidia }}`, `{{ .laptop }}`, etc. Run `dotm init` to re-answer prompts.

### 7. Convert .chezmoiignore to ignore.tmpl

The syntax is nearly identical. Both are rendered as templates, then parsed as glob patterns.

**chezmoi** — `.chezmoiignore`:
```
assets
LICENSE
README.md

{{- if not .goldendict }}
.local/bin/goldendict
.local/share/applications/io.github.xiaoyifang.goldendict_ng.desktop
{{- end }}
```

**dotm** — `ignore.tmpl`:
```
# Repo files
.git/**
assets/**
LICENSE
README.md

{{ if not .goldendict -}}
.local/bin/goldendict
.local/share/applications/io.github.xiaoyifang.goldendict_ng.desktop
{{ end -}}
```

Key difference: dotm uses `**` for recursive directory matching. Single filenames match exactly. chezmoi's ignore patterns and dotm's patterns both support `*`, `?`, and `**`.

### 8. Create the perms file

Map all chezmoi `executable_` and `private_` prefixes to `perms` rules:

```bash
# perms

# pattern                                mode  owner  group

# Executable scripts
.local/bin/**                             0755  -      -
.config/hypr/scripts/**                   0755  -      -
.config/lf/cleaner                        0755  -      -
.config/lf/previewer                      0755  -      -
.config/lf/previewer_sandbox              0755  -      -
.config/lf/vidthumb                       0755  -      -
run_sing-box.sh                           0755  -      -

# Private directories and files (from private_ prefix)
.ssh/                                     0700  -      -
.ssh/**                                   0600  -      -

.config/fcitx5/                           0700  -      -
.config/fcitx5/**/                        0700  -      -
.config/fcitx5/**                         0600  -      -

.local/share/flatpak/                     0700  -      -
.local/share/flatpak/**/                  0700  -      -
.local/share/flatpak/**                   0600  -      -
```

Rules format: `<glob-pattern> <mode|-> <owner|-> <group|->`. Trailing `/` means directories only, no trailing `/` means files only. Last matching rule wins. `-` means don't change that attribute.

### 9. Handle encryption

chezmoi has built-in age encryption configured in `.chezmoi.toml.tmpl`:

```toml
encryption = "age"
[age]
    identity = "~/keys/age/chezmoi.txt"
    recipients = ["age1..."]
```

dotm has no built-in encryption. If your templates already use `output "sops" "-d" ...` for secrets (as in the example), no changes are needed — sops handles decryption at template render time:

```
{{ $s := output "sops" "-d" (joinPath .sourceDir "secrets.enc.yaml") | fromYaml -}}
{{ index $s "api_key" }}
```

Copy `secrets.enc.yaml` and `.sops.yaml` to the new repo root. Ensure `SOPS_AGE_KEY_FILE` is set in your shell environment.

If you used chezmoi's built-in encryption for individual files (not sops), convert them to sops-managed templates or decrypt and store as plain files.

### 10. Remove chezmoi-specific files

Delete any chezmoi-only artifacts from the new repo:

```bash
# Remove chezmoi nvim plugin (if present)
rm -f "$NEW/files/.config/nvim/lua/plugins/chezmoi.lua"

# Remove empty directories (git/dotm don't track them)
find "$NEW/files" -type d -empty -delete
```

If empty directories are needed at the destination, create them via a script:

```toml
# dotm.toml
[[scripts]]
path = "scripts/create_dirs.sh"
template = false
trigger = "always"
```

```bash
#!/bin/bash
# scripts/create_dirs.sh
mkdir -p ~/.config/qt5ct/{colors,qss}
mkdir -p ~/.config/qt6ct/{colors,qss}
```

### 11. Write dotm.toml

Combine all pieces:

```toml
dest = "~"

[prompts]
nvidia       = { type = "bool", question = "NVIDIA GPU" }
amd_cpu      = { type = "bool", question = "AMD CPU (for temp sensors)" }
laptop       = { type = "bool", question = "Laptop (battery, backlight, compact fonts)" }
tablet       = { type = "bool", question = "Drawing tablet (OpenTabletDriver)" }
ocr          = { type = "bool", question = "OCR (transformers_ocr)" }
goldendict   = { type = "bool", question = "GoldenDict" }
subs2srs     = { type = "bool", question = "subs2srs" }
sparrow      = { type = "bool", question = "sparrow-wallet" }
portproton   = { type = "bool", question = "PortProton" }
virt_manager = { type = "bool", question = "QEMU / virt-manager" }

[symlinks]
".local/bin/imv-dir"  = "{{ .homeDir }}/.local/bin/imv"
".local/bin/sing-box" = "{{ .homeDir }}/sing-box/sing-box"

[[scripts]]
path     = "scripts/install_packages.sh.tmpl"
template = true
trigger  = "on_change"

[[scripts]]
path     = "scripts/enable_services.sh.tmpl"
template = true
trigger  = "on_change"
```

### 12. Initialize and verify

```bash
cd ~/dotfiles

# Initialize git
git init && git add -A && git commit -m "migrate from chezmoi to dotm"

# Answer prompts and create state
dotm init

# Dry run — inspect what would happen
dotm apply -n

# Check the output carefully, then apply
dotm apply

# Verify everything matches
dotm status
dotm diff
```

## Final directory structure

```
~/dotfiles/
├── dotm.toml                    # config (was .chezmoi.toml.tmpl)
├── perms                        # permissions (was executable_/private_ prefixes)
├── ignore.tmpl                  # ignore patterns (was .chezmoiignore)
├── secrets.enc.yaml             # sops-encrypted secrets (unchanged)
├── .sops.yaml                   # sops config (unchanged)
├── files/                       # dotfiles with real paths
│   ├── .bashrc                  # was dot_bashrc
│   ├── .zshrc.tmpl              # was dot_zshrc.tmpl
│   ├── .ssh/
│   │   └── config               # was private_dot_ssh/config
│   ├── .config/
│   │   ├── hypr/
│   │   │   ├── hyprland.conf.tmpl
│   │   │   └── scripts/
│   │   │       └── lock.sh      # was executable_lock.sh
│   │   ├── git/
│   │   │   └── config.tmpl
│   │   └── ...
│   └── .local/
│       ├── bin/
│       │   ├── anki.tmpl        # was executable_anki.tmpl
│       │   └── ...
│       └── share/
│           └── applications/
│               └── ...
├── scripts/                     # lifecycle scripts
│   ├── install_packages.sh.tmpl # was run_onchange_install_packages.sh.tmpl
│   └── enable_services.sh.tmpl  # was run_onchange_after_enable_services.sh.tmpl
├── LICENSE
└── README.md
```

## Prefix conversion reference

| chezmoi source path | dotm source path | dotm config |
|---|---|---|
| `dot_zshrc.tmpl` | `files/.zshrc.tmpl` | — |
| `dot_config/git/config.tmpl` | `files/.config/git/config.tmpl` | — |
| `dot_config/hypr/scripts/executable_lock.sh` | `files/.config/hypr/scripts/lock.sh` | `perms`: `0755` |
| `private_dot_ssh/config` | `files/.ssh/config` | `perms`: dir `0700`, file `0600` |
| `dot_config/private_fcitx5/private_conf/private_classicui.conf` | `files/.config/fcitx5/conf/classicui.conf` | `perms`: dir `0700`, file `0600` |
| `dot_local/bin/executable_anki.tmpl` | `files/.local/bin/anki.tmpl` | `perms`: `0755` |
| `dot_local/bin/symlink_imv-dir.tmpl` | *(not a file)* | `[symlinks]` in `dotm.toml` |
| `executable_literal_run_sing-box.sh.tmpl` | `files/run_sing-box.sh.tmpl` | `perms`: `0755` |
| `run_onchange_install_packages.sh.tmpl` | `scripts/install_packages.sh.tmpl` | `[[scripts]]` in `dotm.toml` |
| `.chezmoiignore` | `ignore.tmpl` | — |
| `.chezmoi.toml.tmpl` | `dotm.toml` | — |
