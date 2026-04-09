---
title: DOTM
section: 8
header: System Administration
footer: dotm
---

# NAME

dotm — declarative dotfiles manager

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

**dotm** is a declarative dotfiles manager. It reads configuration from
**dotm.toml**, walks a **files/** directory, renders templates, writes to a
destination directory, applies permissions, and runs lifecycle scripts.

Configuration is read from **dotm.toml**, searched from the current
directory upward. State is stored in **~/.local/state/dotm/**.

Unlike other dotfiles managers, dotm uses normal file paths (no magic
prefixes), delegates encryption to external tools (sops, age), and
supports first-class permission management for system files.

# COMMANDS

**init**
:   Run interactive prompts, resolve template values, and create the state
    cache. Must be run at least once before **apply**. Re-run to re-answer
    prompts.

**apply**
:   Walk **files/**, render templates, write to **dest**, create symlinks,
    apply permissions from the **perms** file, and run lifecycle scripts.
    Records a manifest for orphan detection.

    **-n**, **--dry-run**
    :   Show what would happen without making any changes.

**diff**
:   Show a unified diff between rendered source and current destination
    for each managed file.

**status**
:   Show the sync state of all managed files. Four states are reported:

    **clean**
    :   Destination matches rendered source.

    **modified**
    :   Destination differs from rendered source.

    **missing**
    :   In source but not yet in destination.

    **orphan**
    :   Previously deployed, no longer in source, still in destination.

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

# EXAMPLES

Initialize a new dotfiles repo:

    dotm init

Deploy everything:

    dotm apply

Preview changes:

    dotm apply -n

Check current state:

    dotm status

Show all files including clean:

    dotm status -v

Diff against rendered source:

    dotm diff

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
