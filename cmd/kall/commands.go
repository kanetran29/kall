package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var verbose bool
	var interactive bool

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
  kall -i <command>                  → Run with interactive tab view
  kall -V <command>                  → Run with verbose output
  kall completion <shell>            → Generate shell completions

Options:
  -i, --interactive   Interactive tab view (← → to switch)
  -V, --verbose       Show resolved commands
  -h, --help          Show this help
  -v, --version       Show version

Aliases map a name to different commands per project:
  kall alias frontend start "yarn start"
  kall alias backend start "flask run"
  kall start   # runs the right command in each project`,
		Version:       version,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
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

			results := RunParallel(root, cfg, args)
			RenderResults(results, verbose, interactive)

			for _, r := range results {
				if r.ExitCode != 0 {
					os.Exit(1)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Show commands being executed")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive tab view (← → to switch)")

	cmd.SetHelpTemplate(`{{.Long}}
`)

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAliasCmd())
	cmd.AddCommand(newAliasesCmd())
	cmd.AddCommand(newCompletionCmd())

	return cmd
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

			repos, err := DiscoverRepos(root)
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

			// Build new config, preserving aliases from old config
			newCfg := &Config{}
			for _, name := range selected {
				proj := Project{Name: name, Aliases: make(map[string]string)}
				if existing != nil {
					for _, p := range existing.Projects {
						if p.Name == name {
							proj.Aliases = p.Aliases
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

			hasAliases := false
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
