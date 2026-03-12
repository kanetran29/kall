# kall

Run commands across multiple projects in parallel, with per-project aliases.

```
┌──────────────────────────────────────┐
│ frontend ✓                           │
│──────────────────────────────────────│
│ Compiled successfully.               │
├──────────────────────────────────────┤
│ backend ✓                            │
│──────────────────────────────────────│
│ * Running on http://127.0.0.1:5000   │
└──────────────────────────────────────┘
```

## Install

```bash
# Clone and symlink
git clone git@github.com:kanetran29/kall.git
ln -s "$(pwd)/kall/kall.sh" /usr/local/bin/kall
```

## Quick start

```bash
# Place kall.sh in a parent directory containing your projects:
#   workspace/
#     kall.sh
#     frontend/    ← git repo
#     backend/     ← git repo

kall init              # interactive picker to select projects
kall ls                # run any command across all projects
kall git status        # git works too — just spell it out
```

## Aliases

Map a command name to different commands per project:

```bash
kall alias frontend start "yarn start"
kall alias backend start "flask run"

kall start             # runs the right command in each project
```

Aliases are stored in `.kall` (INI-style):

```ini
[frontend]
start = yarn start
test = yarn test

[backend]
start = flask run
test = pytest
```

## Commands

| Command | Description |
|---|---|
| `kall init` | Scan for git repos and select which to manage |
| `kall config` | Re-select projects (preserves aliases) |
| `kall list` | List configured projects |
| `kall alias <project> <name> <cmd>` | Set a per-project alias |
| `kall aliases` | Show all configured aliases |
| `kall <command> [args]` | Run across all projects in parallel |
| `kall --help` | Show help |
| `kall --version` | Show version |

## How it works

- Runs commands in parallel across all configured projects
- Output is displayed in a stacked box TUI
- Exit codes propagate: ✓ (green) on success, ✗ (red) on failure
- Failed command output is highlighted in red
- If an alias matches the command name, it's resolved per-project
- Extra args after an alias are appended (`kall start --port 3000`)
