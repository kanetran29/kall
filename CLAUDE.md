# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

kall is a Go CLI tool that runs commands across multiple sibling project directories in parallel with a live tabbed TUI. Built with Cobra (CLI framework) and golang.org/x/term (terminal handling). Go 1.26+, no CGO.

## Commands

```bash
make build          # Build binary with version injection
make test           # go test -v ./...
make install        # Build + install to /usr/local/bin + man page
make clean          # Remove built binary

go test -v ./...                            # All tests
go test -v -run TestRunLiveKill ./cmd/kall  # Single test
go vet ./...                                # Static analysis (only linter used)
go run ./cmd/kall                           # Run without installing
```

## Architecture

All source lives in `cmd/kall/` under `package main` — a single flat package, no internal modules.

| File | Purpose |
|---|---|
| `main.go` | Entry point, `version` var (injected via ldflags at build) |
| `commands.go` | All Cobra commands: root, init, list, alias, aliases, completion |
| `config.go` | Config types (`Settings`, `Project`, `Config`), INI-style `.kall` file parsing/writing |
| `runner.go` | `RunParallel` (piped) and `RunLive` (TTY) execution, alias resolution, shell command building |
| `display.go` | `RenderLive` (interactive TUI with alternate screen buffer) and `RenderSequential` (piped output) |
| `workspace.go` | `FindRoot` (walks up dirs for `.kall`), `DiscoverRepos` (finds `.git` subdirs) |
| `picker.go` | Interactive arrow-key project selector TUI, with `pickSimple` CI fallback |
| `proc_unix.go` / `proc_windows.go` | Platform-specific process group management via build tags |

### Key design decisions

- **TTY branching**: `term.IsTerminal(os.Stdout.Fd())` determines live TUI vs sequential piped output
- **Concurrency**: Each `LiveProject` has a `sync.Mutex` guarding its buffer, state, and a generation counter (`gen`) to prevent stale goroutines after rerun
- **Process groups**: Kill sends SIGKILL to entire process group on Unix so `sh -c` children are also killed
- **Config resolution**: per-project alias > global `[*]` alias > pass-through command
- **Shell resolution**: per-project `shell` > `[_settings] shell` > `sh`
- **Flicker-free rendering**: TUI writes entire frame in a single `fmt.Print` call using cursor-home + overwrite (not clear-screen)
- **Environment**: `cleanEnv()` strips `CLAUDECODE` env var from child processes

### Adding a new command

Register with `cmd.AddCommand(newXxxCmd())` inside `newRootCmd()` in `commands.go`.

## Config format (`.kall`)

INI-style with three section types:
- `[_settings]` — `shell`, `concurrency`, `exclude`
- `[*]` — global aliases for all projects
- `[projectname]` — per-project: `label`, `dir`, `shell`, `env.KEY`, alias keys

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`): tests on ubuntu/macos/windows matrix, then auto-bumps semver tag on push to main based on commit message keywords (`[major]`, `[minor]`, default patch). GoReleaser handles cross-compilation, GitHub releases, Homebrew tap, and deb/rpm packages.

Commit with `[skip ci]` in the message to skip the CI pipeline.
