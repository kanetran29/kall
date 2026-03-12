# kall

Run commands across multiple projects in parallel, with per-project aliases.

```
 ▸ frontend ✓ │ backend ✓ │ mobile ✗
──────────────────────────────────────────────
 Compiled successfully.

 ← → switch · q quit
```

Output is shown in an interactive tab UI — use arrow keys to switch between projects.

## Install

### Homebrew (macOS and Linux)

```bash
brew tap kanetran29/tap
brew install kall
```

### From source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/kanetran29/kall.git
cd kall
make install
```

## Quick start

```bash
cd ~/workspace        # parent directory of your projects

kall init             # interactive picker — select which projects to manage
kall ls               # run 'ls' in every project
kall git status       # run any command across all projects
```

## Aliases

Different projects often need different commands for the same task. Aliases let you unify them:

```bash
kall alias frontend start "yarn start"
kall alias backend start "flask run"

kall start            # runs the right command in each project
kall start --port 3000   # extra args are appended
```

Use `-V` to see what actually runs in each project:

```bash
kall -V start
# frontend → $ yarn start
# backend  → $ flask run
```

## Configuration

`kall init` creates a `.kall` file in the current directory. The format is INI-style:

```ini
[frontend]
start = yarn start
test = yarn test

[backend]
start = flask run
test = pytest
```

- Section headers (`[name]`) are project directory names
- Key-value pairs are command aliases
- Lines starting with `#` are comments

kall finds `.kall` by walking up from the working directory (like `.git`), so you can run it from any subdirectory.

## Commands

```
kall init                          → Scan and select projects
kall config                        → Re-select projects
kall list                          → List configured projects
kall alias <project> <name> <cmd>  → Set a command alias
kall aliases                       → List all aliases
kall <command> [args]              → Run across all projects
kall completion <shell>            → Generate shell completions
kall -V <command>                  → Run with verbose (show resolved commands)
kall --version                     → Show version
```

## Shell completions

Homebrew installs completions automatically. For manual setup:

```bash
# Bash
kall completion bash > /usr/local/share/bash-completion/completions/kall

# Zsh
kall completion zsh > "${fpath[1]}/_kall"

# Fish
kall completion fish > ~/.config/fish/completions/kall.fish

# PowerShell
kall completion powershell | Out-File kall.ps1
```

## How it works

1. Commands run in parallel across all configured projects
2. Results are displayed in an interactive tab UI — use **← →** to switch between projects
3. Exit codes propagate: **✓** on success, **✗** on failure
4. If a command name matches an alias, it's resolved per-project
5. When piped (e.g. `kall git status | cat`), output falls back to plain sequential text

## Uninstall

```bash
brew uninstall kall
# or
make uninstall
```

## License

[MIT](LICENSE)
