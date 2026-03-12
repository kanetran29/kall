package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Project represents a project directory and its command aliases.
type Project struct {
	Name    string
	Aliases map[string]string
}

// Config holds the parsed .kall configuration.
type Config struct {
	Projects []Project
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

	cfg := &Config{}
	var current *Project

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if m := sectionRe.FindStringSubmatch(line); m != nil {
			cfg.Projects = append(cfg.Projects, Project{
				Name:    m[1],
				Aliases: make(map[string]string),
			})
			current = &cfg.Projects[len(cfg.Projects)-1]
		} else if current != nil {
			if m := kvRe.FindStringSubmatch(line); m != nil {
				key := strings.TrimSpace(m[1])
				val := strings.TrimSpace(m[2])
				current.Aliases[key] = val
			}
		}
	}

	return cfg, scanner.Err()
}

// WriteConfig writes a Config back to a .kall file.
func WriteConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for i, p := range cfg.Projects {
		if i > 0 {
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "[%s]\n", p.Name)
		for _, k := range sortedKeys(p.Aliases) {
			fmt.Fprintf(f, "%s = %s\n", k, p.Aliases[k])
		}
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
