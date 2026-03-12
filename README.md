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

### Homebrew (macOS and Linux)

```bash
brew tap kanetran29/tap
brew install kall
```

### Debian / Ubuntu

Download the `.deb` from [Releases](https://github.com/kanetran29/kall/releases):

```bash
sudo dpkg -i kall_*.deb
```

### Windows

Download the `.zip` from [Releases](https://github.com/kanetran29/kall/releases) and add to your PATH. Or use scoop:

```powershell
# manual download from GitHub Releases
```

### From source

```bash
git clone https://github.com/kanetran29/kall.git
cd kall
make install          # installs to /usr/local by default
# make PREFIX=~/.local install   # or a custom prefix
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
| `kall completion <shell>` | Generate shell completions (bash/zsh/fish/powershell) |
| `kall --help` | Show help |
| `kall --version` | Show version |

## Shell completions

```bash
# Bash
kall completion bash > /usr/local/share/bash-completion/completions/kall

# Zsh
kall completion zsh > "${fpath[1]}/_kall"

# Fish
kall completion fish > ~/.config/fish/completions/kall.fish

# PowerShell
kall completion powershell > kall.ps1
```

## How it works

1. Commands run in parallel — one goroutine per project
2. Output is collected and displayed in a stacked box TUI
3. Exit codes propagate: green **✓** on success, red **✗** on failure
4. Failed command output is highlighted in red
5. If the command name matches an alias, it's resolved per-project; otherwise the literal command runs

## Uninstall

```bash
brew uninstall kall          # Homebrew
sudo dpkg -r kall            # Debian/Ubuntu
make uninstall               # from source
```

## License

[MIT](LICENSE)
