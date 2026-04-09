---
title: DOTM
section: 8
header: System Administration
footer: dotm
---

# NAME

dotm — declarative dotfiles, packages, and services manager

# SYNOPSIS

**dotm** **init**

**dotm** **apply** [**-n**|**--dry-run**]

**dotm** **diff**

**dotm** **status** [**-v**|**--verbose**] [**-q**|**--quiet**]

**dotm** **reset** *NAME*...

**dotm** **reset** **--all**

**dotm** [**-h**|**--help**]

**dotm** [**-V**|**--version**]

# DESCRIPTION

**dotm** is a declarative dotfiles, packages, and services manager. It
reads configuration from **dotm.toml**, walks a **files/** directory,
renders templates, writes to a destination directory, applies permissions,
runs lifecycle scripts, and manages packages and services via configurable
managers.

Configuration is read from **dotm.toml**, searched from the current
directory upward. State is stored in **~/.local/state/dotm/**.

Unlike other dotfiles managers, dotm uses normal file paths (no magic
prefixes), delegates encryption to external tools (sops, age), supports
first-class permission management for system files, and manages packages
and services with zero hardcoded package managers or init systems.

# COMMANDS

**init**
:   Run interactive prompts, resolve template values, and create the state
    cache. Must be run at least once before **apply**. Re-run to re-answer
    prompts.

**apply**
:   By default, walk **files/**, render templates, write to **dest**, create
    symlinks, apply permissions from the **perms** file, and run lifecycle
    scripts. Records a manifest for orphan detection.

    When scope words are given, operate only on the specified targets:

    **files**
    :   Apply files, symlinks, perms, and scripts (default behavior).

    **pkgs**
    :   Install/remove packages via managers.

    **services**
    :   Enable/disable services via managers.

    **--all**
    :   Apply everything (files + pkgs + services).

    **-n**, **--dry-run**
    :   Show what would happen without making any changes.

**diff**
:   By default, show a unified diff between rendered source and current
    destination for each managed file. With scope words, also show package
    and service changes:

    **files**
    :   Show file diffs only (default).

    **pkgs**
    :   Show packages that would be installed/removed.

    **services**
    :   Show services that would be enabled/disabled.

    **--all**
    :   Show all diffs (files + pkgs + services).

**status**
:   By default, show the sync state of all managed files. With scope words,
    show package and service status:

    **files**
    :   Show file sync state only (default).

    **pkgs**
    :   Show package status only.

    **services**
    :   Show service status only.

    **--all**
    :   Show everything (files + pkgs + services).

    Four states are reported for files:

    **clean**
    :   Destination matches rendered source.

    **modified**
    :   Destination differs from rendered source.

    **missing**
    :   In source but not yet in destination.

    **orphan**
    :   Previously deployed, no longer in source, still in destination.

    Packages report as **OK**, **MISSING**, or **OBSOLETE**.
    Services report as **ENABLED**, **DISABLED**, or **OBSOLETE**.

    **-v**, **--verbose**
    :   Include clean files in the output.

    **-q**, **--quiet**
    :   Exit 1 if any problems exist, produce no output.

**reset**
:   Reset cached prompt value(s). After resetting, run **dotm init** to
    re-answer prompts.

    *NAME* [*NAME*...]
    :   Reset specific prompt(s). Errors if a name is not found.

    **--all**
    :   Reset all cached values.

# OPTIONS

**-h**, **--help**
:   Show usage information and exit.

**-V**, **--version**
:   Print program name and version, then exit.

# CONFIGURATION

Create **dotm.toml** in your dotfiles repository.

```toml
dest = "~"

[prompts]
use_nvidia = { type = "bool", question = "Enable NVIDIA config?" }
git_email  = { type = "string", question = "Git email address" }

[symlinks]
".local/bin/editor" = "{{ .homeDir }}/.nix-profile/bin/nvim"

[[scripts]]
path = "scripts/reload.sh"
template = true
trigger = "on_change"
```

## Options

**dest**
:   Destination directory. Use **"/"** for system-wide configuration.
    Supports **~** prefix for home directory. Default: **~**.

**shell**
:   Shell for script execution. Default: **bash**.

**[prompts]**
:   Interactive prompts with values available in templates. Types:
    **bool** or **string**.

**[symlinks]**
:   Map of symlink paths (relative to **dest**) to targets. Targets may
    contain template expressions.

**[[scripts]]**
:   Lifecycle scripts to run after apply.
    **path** — relative to source directory.
    **template** — render as template before running. Default: **false**.
    **trigger** — **always** or **on_change**. Default: **always**.

## Built-in template variables

| Variable | Value |
|----------|-------|
| **{{ .homeDir }}** | User home directory |
| **{{ .hostname }}** | System hostname |
| **{{ .username }}** | Current user |
| **{{ .sourceDir }}** | Absolute path to dotfiles repo |

## Custom template functions

**output "cmd" "arg1" "arg2"**
:   Run command, return stdout.

**fromYaml**
:   Parse YAML string into map.

**joinPath "a" "b"**
:   Path joining (filepath.Join).

**hasKey $map "key"**
:   Check if a map contains a key.

**replace "old" "new" $s**
:   Replace all occurrences in a string.

**default "fallback" $value**
:   Return fallback if value is empty or nil.

## Ignore patterns

The **ignore.tmpl** file (rendered as template, then parsed as glob
patterns) controls which files are skipped during apply and status.
Patterns are matched against paths relative to **files/**. Supports
**\***, **?**, **\*\***.

## Permissions

The **perms** file defines permission rules for deployed files:

```bash
# pattern              mode   owner  group
etc/**/                0755   root   root
etc/**                 0644   root   root
etc/security/          0700   root   root
```

Trailing **/** matches directories only. No trailing **/** matches files
only. **-** means don't change that attribute. Last matching rule wins.

## Package and service management

Packages and services are managed via declarative **managers**. A manager
defines command templates for **check**, **install**, **remove**, **enable**,
and **disable**. Groups (e.g. **[pacman]**, **[systemd]**) reference a
manager by name and list packages or services.

```toml
[managers.pacman]
check   = "pacman -Q {{.Name}}"
install = "sudo pacman -S --needed {{.Name}}"
remove  = "sudo pacman -Rns {{.Name}}"

[managers.systemd]
check   = "systemctl is-enabled {{.Name}}"
enable  = "sudo systemctl enable {{.Name}}"
disable = "sudo systemctl disable {{.Name}}"

[pacman]
packages = ["git", "neovim", "{{ if .laptop }}brightnessctl{{ end }}"]

[systemd]
services = ["firewalld", "sshd"]
```

Package and service names may contain Go template expressions. If a name
renders to an empty string, the entry is skipped.

Template variables for commands:
- **{{.Name}}** — raw package/service name
- In check/install/remove/enable/disable: **{{.Name}}** is shell-quoted

# EXAMPLES

Initialize a new dotfiles repo:

    dotm init

Deploy files, symlinks, perms, and scripts:

    dotm apply

Deploy everything including packages and services:

    dotm apply --all

Preview changes:

    dotm apply -n

Check current state of files:

    dotm status

Check package status:

    dotm status pkgs

Show all files including clean:

    dotm status -v

Show everything:

    dotm status --all

Diff against rendered source:

    dotm diff

Show what packages would be installed/removed:

    dotm diff pkgs

# EXIT STATUS

**0**
:   Success.

**1**
:   Error. Common causes: no command specified, config parse failure,
    template error, permission apply failure, script execution failure.

# FILES

**~/.local/state/dotm/\*.toml**
:   Per-source-repo state files (prompt answers, script hashes, manifest).
    Filename is SHA-256 hash of the source directory path.

# SEE ALSO

**gitpkg**(1)
