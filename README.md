# kall

Run commands across multiple projects in parallel, with per-project aliases.

```
┌──────────────────────────────────────────┐
│ frontend ✓                               │
│──────────────────────────────────────────│
│ Compiled successfully.                   │
├──────────────────────────────────────────┤
│ backend ✓                                │
│──────────────────────────────────────────│
│ * Running on http://127.0.0.1:5000       │
└──────────────────────────────────────────┘
```

## Install

### Homebrew

```bash
brew tap kanetran29/tap
brew install kall
```

### From source

```bash
git clone https://github.com/kanetran29/kall.git
cd kall
make install          # installs to /usr/local by default
# make PREFIX=~/.local install   # or a custom prefix
```

### Manual

Copy the script and optionally the completions:

```bash
cp bin/kall /usr/local/bin/
cp completions/kall.bash /usr/local/share/bash-completion/completions/kall
cp completions/_kall /usr/local/share/zsh/site-functions/
```

## Quick start

```bash
cd ~/workspace        # parent directory of your projects

kall init             # interactive picker — select which projects to manage
kall ls               # run 'ls' in every project
kall git status       # git works too, just spell it out
```

## Aliases

Different projects often need different commands for the same task. Aliases let you unify them:

```bash
kall alias frontend start "yarn start"
kall alias backend start "flask run"

kall start            # runs the right command in each project
kall start --port 3000   # extra args are appended
```

## Configuration

`kall init` creates a `.kall` file in the current directory. The format is INI-style:

```ini
# Frontend app
[frontend]
start = yarn start
test = yarn test
build = yarn build

# Backend API
[backend]
start = flask run
test = pytest
build = docker build -t api .
```

- Section headers (`[name]`) are project directory names
- Key-value pairs are command aliases
- Lines starting with `#` are comments

kall finds `.kall` by walking up from the working directory (like `.git`), so you can run kall from any subdirectory within the workspace.

## Commands

| Command | Description |
|---|---|
| `kall init` | Scan for git repos, interactively select projects |
| `kall config` | Re-select projects (preserves existing aliases) |
| `kall list` | List configured projects |
| `kall alias <project> <name> <cmd>` | Set a per-project command alias |
| `kall aliases` | Show all configured aliases |
| `kall <command> [args]` | Run across all projects in parallel |
| `kall --help` | Show help |
| `kall --version` | Show version |

## How it works

1. Commands run in parallel — one background process per project
2. Output is collected and displayed in a stacked box TUI
3. Exit codes propagate: green **✓** on success, red **✗** on failure
4. Failed command output is highlighted in red
5. If the command name matches an alias, it's resolved per-project; otherwise the literal command runs

## Uninstall

```bash
brew uninstall kall          # Homebrew
# or
make uninstall               # from source
```

## License

[MIT](LICENSE)
