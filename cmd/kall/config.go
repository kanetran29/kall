package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Settings holds global kall configuration from the [_settings] section.
type Settings struct {
	Shell       string   // default shell for commands (e.g. /bin/zsh)
	Concurrency int      // max parallel jobs (0 = unlimited)
	Exclude     []string // patterns to exclude from kall init discovery
}

// Project represents a project directory and its configuration.
type Project struct {
	Name    string
	Label   string            // display name for tabs (defaults to Name)
	Dir     string            // subdirectory to run commands in (relative to project root)
	Shell   string            // per-project shell override
	Env     map[string]string // per-project environment variables
	Aliases map[string]string // command aliases
}

// DisplayName returns Label if set, otherwise Name.
func (p Project) DisplayName() string {
	if p.Label != "" {
		return p.Label
	}
	return p.Name
}

// Config holds the parsed .kall configuration.
type Config struct {
	Settings      Settings
	GlobalAliases map[string]string // [*] section — applies to all projects
	Projects      []Project
}

var (
	sectionRe = regexp.MustCompile(`^\[(.+)\]$`)
	kvRe      = regexp.MustCompile(`^([^=]+)=(.+)$`)
)

// ParseConfig reads and parses a .kall INI-style config file.
func ParseConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{
		GlobalAliases: make(map[string]string),
	}

	var currentSection string // "_settings", "*", or project name
	var currentProject *Project

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if m := sectionRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			currentSection = name
			currentProject = nil

			switch name {
			case "_settings":
				// handled below via currentSection
			case "*":
				// handled below via currentSection
			default:
				cfg.Projects = append(cfg.Projects, Project{
					Name:    name,
					Env:     make(map[string]string),
					Aliases: make(map[string]string),
				})
				currentProject = &cfg.Projects[len(cfg.Projects)-1]
			}
			continue
		}

		m := kvRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := strings.TrimSpace(m[1])
		val := strings.TrimSpace(m[2])

		switch currentSection {
		case "_settings":
			parseSettingsKV(&cfg.Settings, key, val)
		case "*":
			cfg.GlobalAliases[key] = val
		default:
			if currentProject != nil {
				parseProjectKV(currentProject, key, val)
			}
		}
	}

	return cfg, scanner.Err()
}

func parseSettingsKV(s *Settings, key, val string) {
	switch key {
	case "shell":
		s.Shell = val
	case "concurrency":
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			s.Concurrency = n
		}
	case "exclude":
		for _, part := range strings.Split(val, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				s.Exclude = append(s.Exclude, part)
			}
		}
	}
}

func parseProjectKV(p *Project, key, val string) {
	switch {
	case key == "label":
		p.Label = val
	case key == "dir":
		p.Dir = val
	case key == "shell":
		p.Shell = val
	case strings.HasPrefix(key, "env."):
		envKey := strings.TrimPrefix(key, "env.")
		p.Env[envKey] = val
	default:
		p.Aliases[key] = val
	}
}

// WriteConfig writes a Config back to a .kall file.
func WriteConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	needBlank := false

	// Write [_settings] if any are set
	if cfg.Settings.Shell != "" || cfg.Settings.Concurrency > 0 || len(cfg.Settings.Exclude) > 0 {
		fmt.Fprintln(f, "[_settings]")
		if cfg.Settings.Shell != "" {
			fmt.Fprintf(f, "shell = %s\n", cfg.Settings.Shell)
		}
		if cfg.Settings.Concurrency > 0 {
			fmt.Fprintf(f, "concurrency = %d\n", cfg.Settings.Concurrency)
		}
		if len(cfg.Settings.Exclude) > 0 {
			fmt.Fprintf(f, "exclude = %s\n", strings.Join(cfg.Settings.Exclude, ", "))
		}
		needBlank = true
	}

	// Write [*] global aliases
	if len(cfg.GlobalAliases) > 0 {
		if needBlank {
			fmt.Fprintln(f)
		}
		fmt.Fprintln(f, "[*]")
		for _, k := range sortedKeys(cfg.GlobalAliases) {
			fmt.Fprintf(f, "%s = %s\n", k, cfg.GlobalAliases[k])
		}
		needBlank = true
	}

	// Write project sections
	for _, p := range cfg.Projects {
		if needBlank {
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "[%s]\n", p.Name)

		if p.Label != "" {
			fmt.Fprintf(f, "label = %s\n", p.Label)
		}
		if p.Dir != "" {
			fmt.Fprintf(f, "dir = %s\n", p.Dir)
		}
		if p.Shell != "" {
			fmt.Fprintf(f, "shell = %s\n", p.Shell)
		}
		for _, k := range sortedKeys(p.Env) {
			fmt.Fprintf(f, "env.%s = %s\n", k, p.Env[k])
		}
		for _, k := range sortedKeys(p.Aliases) {
			fmt.Fprintf(f, "%s = %s\n", k, p.Aliases[k])
		}

		needBlank = true
	}

	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
