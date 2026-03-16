# Changelog

## 2.1.1 — 2026-03-16

Bug fixes for TUI responsiveness and argument handling.

- Auto-exit TUI when all commands finish (fast commands like `git checkout` no longer appear stuck)
- Print results sequentially after TUI exits so output persists on screen
- Fix stale content showing between tabs when switching from longer to shorter output
- Preserve `--` in commands (no longer stripped by flag parser)

## 2.1.0 — 2026-03-12

Live streaming tab UI with interactive controls and full config system.

- Live streaming output — real-time logs in each tab, not just "Running..."
- Kill (`x`) and rerun (`r`) keybindings per tab
- Flicker-free TUI rendering (cursor-home + line-overwrite)
- Full config system: `[_settings]`, `[*]` global aliases, per-project label/dir/shell/env
- Concurrency limiting via `concurrency` setting
- Exclude list for `kall init` via `exclude` setting
- Tab UI is now the default for TTY (no `-i` flag needed)
- Shell execution for aliases (supports env vars, pipes, shell syntax)
- Strips `CLAUDECODE` env var from child processes
- Process group kill on Unix for clean process tree teardown

## 2.0.0 — 2026-03-12

Complete rewrite in Go for cross-platform support.

- Cross-platform: native binaries for macOS, Linux, and Windows
- Parallel execution using goroutines
- Same stacked box TUI with color-coded exit status
- Same `.kall` config format (fully backwards-compatible)
- Shell completions for bash, zsh, fish, and PowerShell
- GoReleaser pipeline: Homebrew, deb, rpm, Windows zip
- CI: tests run on macOS, Linux, and Windows
- Unit tests for config parsing, workspace discovery, runner, and display

## 1.0.0 — 2026-03-12

Initial release (bash).

- Run any command across multiple projects in parallel
- Stacked box TUI with color-coded exit status
- Per-project command aliases (`kall alias frontend start "yarn start"`)
- INI-style `.kall` config with comment support
- Interactive project picker (`kall init`)
- Bash and Zsh completions
- Man page
