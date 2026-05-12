package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// shellCandidates collects autocompletable first-token names: built-ins,
// subcommands, global aliases, and per-project aliases.
func shellCandidates(root string) []string {
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
	}

	for _, s := range []string{"exit", "quit", "q", "clear", "cls", "help"} {
		add(s)
	}
	tmp := newRootCmd()
	for _, c := range tmp.Commands() {
		add(c.Name())
		for _, alias := range c.Aliases {
			add(alias)
		}
	}
	if root != "" {
		if cfg, err := ParseConfig(filepath.Join(root, ".kall")); err == nil {
			for k := range cfg.GlobalAliases {
				add(k)
			}
			for _, p := range cfg.Projects {
				for k := range p.Aliases {
					add(k)
				}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	// Sort deterministically for stable completion display.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// appendHistory adds a line to history, deduping consecutive repeats and
// capping size.
func appendHistory(h []string, line string) []string {
	const maxHistory = 500
	if len(h) > 0 && h[len(h)-1] == line {
		return h
	}
	h = append(h, line)
	if len(h) > maxHistory {
		h = h[len(h)-maxHistory:]
	}
	return h
}

// inShell is true while the REPL loop is active. Used to suppress process exit
// on non-zero command status so the shell can keep running.
var inShell bool

func printBanner() {
	green := "\033[32m"
	bold := "\033[1m"
	dim := "\033[2m"
	reset := "\033[0m"
	fmt.Fprintf(os.Stderr, bold+green+`
  ██╗  ██╗ █████╗ ██╗     ██╗
  ██║ ██╔╝██╔══██╗██║     ██║
  █████╔╝ ███████║██║     ██║
  ██╔═██╗ ██╔══██║██║     ██║
  ██║  ██╗██║  ██║███████╗███████╗
  ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝
`+reset+dim+"  $_ run commands across all projects"+reset+" %s\n\n", version)
}

func newRootCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "kall [command] [args...]",
		Short: "Run commands across multiple projects in parallel",
		Long: `kall — run commands across multiple projects in parallel

Usage:
  kall init                          → Scan and select projects
  kall config                        → Re-select projects
  kall list                          → List configured projects
  kall alias <project> <name> <cmd>  → Set a command alias
  kall aliases                       → List all aliases
  kall <command> [args]              → Run across all projects
  kall -V <command>                  → Run with verbose output
  kall shell                         → Interactive REPL (no retyping 'kall')
  kall completion <shell>            → Generate shell completions

Options:
  -V, --verbose       Show resolved commands
  -h, --help          Show this help
  -v, --version       Show version

Tab UI (TTY):
  ← →  switch tabs    r  rerun     x  kill     q  quit
  Piped output falls back to sequential plain text.

Config (.kall):
  [_settings]                        → Global settings
    shell = /bin/zsh                   Default shell for commands
    concurrency = 2                    Max parallel jobs
    exclude = node_modules, dist       Hide from kall init

  [*]                                → Global aliases (all projects)
    test = npm test

  [project]                          → Per-project config
    label = FE                         Short name for tabs
    dir = src/app                      Subdirectory to run in
    shell = /bin/bash                  Shell override
    env.PORT = 3000                    Environment variable
    start = yarn start                 Command alias`,
		Version:       version,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				printBanner()
				return cmd.Help()
			}

			root, err := FindRoot()
			if err != nil {
				return err
			}

			cfg, err := ParseConfig(filepath.Join(root, ".kall"))
			if err != nil {
				return err
			}

			if len(cfg.Projects) == 0 {
				return fmt.Errorf("no projects in .kall config. Run 'kall init' first")
			}

			var results []Result
			accent := resolveAccent(cfg.Settings.Color)

			if term.IsTerminal(int(os.Stdout.Fd())) {
				// TTY: live tab UI — shows tabs immediately, streams output in real-time
				lives, doneCh := RunLive(root, cfg, args)
				results = RenderLive(lives, doneCh, verbose, accent)
				// Print results after TUI exits so output persists on screen
				RenderSequential(results, verbose, accent)
			} else {
				// Piped: run all, then print sequentially
				results = RunParallel(root, cfg, args)
				RenderSequential(results, verbose, accent)
			}

			for _, r := range results {
				if r.ExitCode != 0 {
					if inShell {
						return nil
					}
					os.Exit(1)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Show commands being executed")
	cmd.Flags().SetInterspersed(false) // stop flag parsing after first positional arg so -- is passed through

	cmd.SetHelpTemplate(`{{.Long}}
`)

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAliasCmd())
	cmd.AddCommand(newAliasesCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newShellCmd())

	return cmd
}

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Interactive REPL — run kall commands without retyping 'kall'",
		Long: `kall shell — interactive REPL

Reads commands line by line and dispatches each as if you typed 'kall <line>'.
Useful when running many commands across the same project set.

Built-ins:
  exit, quit, q     Leave the shell
  clear, cls        Clear the screen
  help              Show kall help

End with Ctrl+D (EOF) or Ctrl+C.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inShell {
				return errors.New("already in a kall shell")
			}
			return runShellLoop()
		},
	}
}

func runShellLoop() error {
	inShell = true
	defer func() { inShell = false }()

	accent := "\033[32m"
	bold := "\033[1m"
	dim := "\033[2m"
	red := "\033[31m"
	reset := "\033[0m"

	root, err := FindRoot()
	rootLabel := "?"
	if err == nil {
		rootLabel = filepath.Base(root)
	}

	fmt.Printf("%s%skall shell%s %s(root: %s — type 'exit' or Ctrl+D to leave)%s\n",
		bold, accent, reset, dim, rootLabel, reset)

	editor := &lineEditor{
		prompt:     fmt.Sprintf("%skall%s%s>%s ", accent, dim, reset, reset),
		candidates: shellCandidates(root),
	}

	for {
		line, err := editor.readLine()
		if err == io.EOF {
			return nil
		}
		if errors.Is(err, errInterrupted) {
			continue
		}
		if err != nil {
			return err
		}

		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		editor.history = appendHistory(editor.history, line)

		switch line {
		case "exit", "quit", "q":
			return nil
		case "clear", "cls":
			fmt.Print("\033[H\033[2J")
			continue
		}

		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}
		if args[0] == "shell" {
			fmt.Fprintf(os.Stderr, "%salready in shell%s\n", red, reset)
			continue
		}

		sub := newRootCmd()
		sub.SetArgs(args)
		sub.SetOut(os.Stdout)
		sub.SetErr(os.Stderr)
		if err := sub.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", red, err.Error(), reset)
		}
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Aliases: []string{"config"},
		Short:   "Scan and select projects to manage",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := FindRoot()
			if err != nil {
				root, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			configPath := filepath.Join(root, ".kall")

			// Load existing config to preserve aliases
			existing, _ := ParseConfig(configPath)

			var exclude []string
			if existing != nil {
				exclude = existing.Settings.Exclude
			}
			repos, err := DiscoverRepos(root, exclude)
			if err != nil {
				return err
			}

			if len(repos) == 0 {
				return fmt.Errorf("no git repos found in %s", root)
			}

			// Build currently selected list
			var currentSelected []string
			if existing != nil {
				for _, p := range existing.Projects {
					currentSelected = append(currentSelected, p.Name)
				}
			}

			selected, err := PickProjects(repos, currentSelected)
			if err != nil {
				return err
			}

			if len(selected) == 0 {
				fmt.Println("No projects selected.")
				os.Remove(configPath)
				return nil
			}

			// Build new config, preserving all settings from old config
			newCfg := &Config{
				GlobalAliases: make(map[string]string),
			}
			if existing != nil {
				newCfg.Settings = existing.Settings
				newCfg.GlobalAliases = existing.GlobalAliases
			}
			for _, name := range selected {
				proj := Project{Name: name, Env: make(map[string]string), Aliases: make(map[string]string)}
				if existing != nil {
					for _, p := range existing.Projects {
						if p.Name == name {
							proj = p // preserve everything
							break
						}
					}
				}
				newCfg.Projects = append(newCfg.Projects, proj)
			}

			if err := WriteConfig(configPath, newCfg); err != nil {
				return err
			}

			fmt.Printf("Saved %d project(s) to .kall\n", len(selected))
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := FindRoot()
			if err != nil {
				return err
			}

			cfg, err := ParseConfig(filepath.Join(root, ".kall"))
			if err != nil {
				return err
			}

			for _, p := range cfg.Projects {
				fmt.Println(p.Name)
			}
			return nil
		},
	}
}

func newAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "alias <project> <name> <command...>",
		Short: "Set a command alias for a project",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			name := args[1]
			command := strings.Join(args[2:], " ")

			root, err := FindRoot()
			if err != nil {
				return err
			}

			configPath := filepath.Join(root, ".kall")
			cfg, err := ParseConfig(configPath)
			if err != nil {
				return err
			}

			found := false
			for i, p := range cfg.Projects {
				if p.Name == project {
					cfg.Projects[i].Aliases[name] = command
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("project '%s' not found in .kall", project)
			}

			if err := WriteConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Set alias: %s \u2192 %s = %s\n", project, name, command)
			return nil
		},
	}
}

func newAliasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "aliases",
		Short: "List all configured aliases",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := FindRoot()
			if err != nil {
				return err
			}

			cfg, err := ParseConfig(filepath.Join(root, ".kall"))
			if err != nil {
				return err
			}

			hasAliases := len(cfg.GlobalAliases) > 0
			for _, p := range cfg.Projects {
				if len(p.Aliases) > 0 {
					hasAliases = true
					break
				}
			}

			if !hasAliases {
				fmt.Println("No aliases configured.")
				return nil
			}

			first := true

			if len(cfg.GlobalAliases) > 0 {
				fmt.Println("[*] (global)")
				for _, k := range sortedKeys(cfg.GlobalAliases) {
					fmt.Printf("  %s = %s\n", k, cfg.GlobalAliases[k])
				}
				first = false
			}

			for _, p := range cfg.Projects {
				if len(p.Aliases) == 0 {
					continue
				}
				if !first {
					fmt.Println()
				}
				fmt.Printf("[%s]\n", p.Name)
				for _, k := range sortedKeys(p.Aliases) {
					fmt.Printf("  %s = %s\n", k, p.Aliases[k])
				}
				first = false
			}
			return nil
		},
	}
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
