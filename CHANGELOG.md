# Changelog

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
