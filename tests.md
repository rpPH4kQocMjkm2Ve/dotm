# Tests

## Overview

| Package | File | What it tests |
|---------|------|---------------|
| `internal/config` | `config_test.go` | `expandHome` (tilde, tilde+path, absolute, relative, empty), `validate` (dest required, prompt types bool/string, unknown type, missing question, script path required, trigger default/on_change/unknown), `Load` (minimal, custom shell, tilde expansion, missing file, invalid TOML, missing dest, prompts, scripts) |
| `internal/engine` | `status_test.go` | `Status` (clean/modified/missing/orphan files, orphan not in dest skipped, template rendering, no source dir, mixed statuses, sorted output, nested paths), `HasProblems`, `Counts` |
| `internal/ignore` | `ignore_test.go` | `matchGlob` (exact, star, double-star suffix/prefix/middle/zero/deep, question mark, match-everything), `Match` (empty/single/multiple patterns), `Load` (no file, patterns with comments, template conditional true/false, comments and blanks skipped) |
| `internal/perms` | `parse_test.go` | `ParseRules` (empty, comments, blanks, single file/dir, skip marker, partial skip, multiple ordering, invalid modes, field count, empty pattern, line numbers, trailing slash) |
| `internal/perms` | `glob_test.go` | `MatchGlob` (globstar nested/direct/start/middle/zero/many, single star, question mark, exact match, regex metacharacter escaping, edge cases) |
| `internal/perms` | `apply_test.go` | `ComputeActions` (empty rules/managed, single match, dir-only/file-only rules, last-match-wins, skip mode, non-root dest, outside dest, no match, mixed files+dirs, specific overrides, dest dir skipped, deeply nested, specific group), `ApplyActions` — chmod file/dir/restrictive/nonexistent/skip-mode, chown root/non-root/group-only/nonexistent-user, combined chmod+chown, partial failure, dry run (**requires root**), full pipeline, idempotency, PAM safety regression |
| `internal/prompt` | `prompt_test.go` | `coerceValue` (bool, int64, float64 whole/fractional, string true/false/regular, unknown type), `HashContent` (deterministic, different content, empty), `FormatPromptValue` (bool, string, other), `Resolve` (no prompts, cached skip, bool yes/no, string, retry invalid input, multiple prompts sorted, EOF error), `SetManifest` (sorting), `ResetPrompt`, `GetScriptHash`/`SetScriptHash` |
| `internal/safetemp` | `safetemp_test.go` | `SecureDir` (directory creation with 0700, XDG_RUNTIME_DIR priority, home fallback, file creation), `secureDirs` (order, XDG vs home), reuse of existing directory, graceful fallback when all paths unwritable |
| `internal/tmpl` | `tmpl_test.go` | `Render` (plain text, variable substitution, missing key error, conditional true/false, invalid template), `RenderFile` (normal, missing file), template functions: `joinPath`, `hasKey` exists/missing, `replace`, `fromYaml` valid/invalid, `output` echo/trim-newline/nonexistent-command |

## Running

```bash
# All tests (no root)
make test

# Individual package
go test ./internal/config/ -v
go test ./internal/engine/ -v
go test ./internal/ignore/ -v
go test ./internal/perms/ -v      # skips root-only tests
go test ./internal/prompt/ -v
go test ./internal/tmpl/ -v

# Perms tests that require root (chmod/chown/full pipeline)
make test-root
```

## How they work

### Unit tests

All tests use Go's standard `testing` package with `t.TempDir()` for filesystem isolation. No external test frameworks.

**`config`** — tests `expandHome` directly as an unexported function (same package). `validate` is tested by constructing `Config` structs with various invalid states. `Load` tests write temporary TOML files and verify parsing, default values, and error paths.

**`engine`** — `status_test.go` creates a minimal source+dest tree in temp directories, writes a `dotm.toml` with the dest path, builds an `Engine` via the public `New` constructor, and calls `Status()`. Templates (`.tmpl` files) are tested by comparing rendered output against dest content using the real hostname from `os.Hostname()`.

**`ignore`** — `matchGlob` is tested directly as an unexported function. `Load` tests write `ignore.tmpl` files with template conditionals and verify that patterns are included/excluded based on template data.

**`perms`** — three test files covering the parse→compute→apply pipeline. `parse_test.go` and `glob_test.go` run without privileges. `apply_test.go` contains both unprivileged `ComputeActions` tests and root-only `ApplyActions` tests guarded by `skipIfNotRoot`. The `isDirFunc` parameter in `ComputeActions` is injected in tests to avoid real filesystem lookups. Root tests use `syscall.Stat` to verify actual permission bits and ownership after apply.

**`prompt`** — `Resolve` tests inject `strings.NewReader` as stdin and `bytes.Buffer` as stdout, simulating interactive input without a terminal. Multiple prompts are verified to execute in sorted key order.

**`tmpl`** — template function tests render templates with known data and compare output. `output` tests use `echo` as a portable command. `fromYaml` tests verify both valid parsing and error handling for malformed YAML.

## Test environment

- All tests create temporary directories via `t.TempDir()`, cleaned up automatically
- No root privileges required except `internal/perms` apply tests (chmod/chown)
- No real home directories, dotfile repos, or system files are touched
- Root-only tests skip with `t.Skip("requires root")` when run as non-root
- `go test ./...` runs everything safe; `make test-root` runs perms tests under sudo
