package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := `# Frontend app
[frontend]
start = yarn start
test = yarn test

# Backend API
[backend]
start = flask run
test = pytest
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}

	if cfg.Projects[0].Name != "frontend" {
		t.Errorf("expected project 'frontend', got '%s'", cfg.Projects[0].Name)
	}
	if cfg.Projects[0].Aliases["start"] != "yarn start" {
		t.Errorf("expected alias 'yarn start', got '%s'", cfg.Projects[0].Aliases["start"])
	}
	if cfg.Projects[0].Aliases["test"] != "yarn test" {
		t.Errorf("expected alias 'yarn test', got '%s'", cfg.Projects[0].Aliases["test"])
	}

	if cfg.Projects[1].Name != "backend" {
		t.Errorf("expected project 'backend', got '%s'", cfg.Projects[1].Name)
	}
	if cfg.Projects[1].Aliases["start"] != "flask run" {
		t.Errorf("expected alias 'flask run', got '%s'", cfg.Projects[1].Aliases["start"])
	}
}

func TestParseConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(cfg.Projects))
	}
}

func TestParseConfigSectionsOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := "[frontend]\n\n[backend]\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if len(cfg.Projects[0].Aliases) != 0 {
		t.Errorf("expected 0 aliases, got %d", len(cfg.Projects[0].Aliases))
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	cfg := &Config{
		GlobalAliases: make(map[string]string),
		Projects: []Project{
			{Name: "frontend", Env: make(map[string]string), Aliases: map[string]string{"start": "yarn start"}},
			{Name: "backend", Env: make(map[string]string), Aliases: map[string]string{"start": "flask run", "test": "pytest"}},
		},
	}

	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "[frontend]") || !strings.Contains(content, "[backend]") {
		t.Errorf("missing section headers in output:\n%s", content)
	}
	if !strings.Contains(content, "start = yarn start") {
		t.Errorf("missing frontend alias in output:\n%s", content)
	}
	if !strings.Contains(content, "start = flask run") {
		t.Errorf("missing backend alias in output:\n%s", content)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	original := &Config{
		GlobalAliases: make(map[string]string),
		Projects: []Project{
			{Name: "app", Env: make(map[string]string), Aliases: map[string]string{"build": "make build", "test": "make test"}},
			{Name: "lib", Env: make(map[string]string), Aliases: map[string]string{}},
		},
	}

	if err := WriteConfig(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Projects) != len(original.Projects) {
		t.Fatalf("project count mismatch: %d vs %d", len(loaded.Projects), len(original.Projects))
	}

	for i, p := range loaded.Projects {
		if p.Name != original.Projects[i].Name {
			t.Errorf("project %d: name %q != %q", i, p.Name, original.Projects[i].Name)
		}
		for k, v := range original.Projects[i].Aliases {
			if p.Aliases[k] != v {
				t.Errorf("project %d alias %q: %q != %q", i, k, p.Aliases[k], v)
			}
		}
	}
}

func TestParseConfigSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := `[_settings]
shell = /bin/zsh
concurrency = 3
exclude = node_modules, dist

[frontend]
start = yarn start
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Settings.Shell != "/bin/zsh" {
		t.Errorf("expected shell '/bin/zsh', got '%s'", cfg.Settings.Shell)
	}
	if cfg.Settings.Concurrency != 3 {
		t.Errorf("expected concurrency 3, got %d", cfg.Settings.Concurrency)
	}
	if len(cfg.Settings.Exclude) != 2 || cfg.Settings.Exclude[0] != "node_modules" || cfg.Settings.Exclude[1] != "dist" {
		t.Errorf("expected exclude [node_modules, dist], got %v", cfg.Settings.Exclude)
	}
}

func TestParseConfigGlobalAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := `[*]
test = npm test
lint = npm run lint

[frontend]
test = yarn test
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GlobalAliases["test"] != "npm test" {
		t.Errorf("expected global alias 'npm test', got '%s'", cfg.GlobalAliases["test"])
	}
	if cfg.GlobalAliases["lint"] != "npm run lint" {
		t.Errorf("expected global alias 'npm run lint', got '%s'", cfg.GlobalAliases["lint"])
	}
	// Project-level should override
	if cfg.Projects[0].Aliases["test"] != "yarn test" {
		t.Errorf("expected project alias 'yarn test', got '%s'", cfg.Projects[0].Aliases["test"])
	}
}

func TestParseConfigProjectExtras(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := `[frontend]
label = FE
dir = src/app
shell = /bin/bash
env.PORT = 3000
env.HOST = localhost
start = yarn start
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	p := cfg.Projects[0]
	if p.Label != "FE" {
		t.Errorf("expected label 'FE', got '%s'", p.Label)
	}
	if p.Dir != "src/app" {
		t.Errorf("expected dir 'src/app', got '%s'", p.Dir)
	}
	if p.Shell != "/bin/bash" {
		t.Errorf("expected shell '/bin/bash', got '%s'", p.Shell)
	}
	if p.Env["PORT"] != "3000" {
		t.Errorf("expected env PORT=3000, got '%s'", p.Env["PORT"])
	}
	if p.Env["HOST"] != "localhost" {
		t.Errorf("expected env HOST=localhost, got '%s'", p.Env["HOST"])
	}
	if p.Aliases["start"] != "yarn start" {
		t.Errorf("expected alias 'yarn start', got '%s'", p.Aliases["start"])
	}
}

func TestDisplayName(t *testing.T) {
	p1 := Project{Name: "frontend"}
	if p1.DisplayName() != "frontend" {
		t.Errorf("expected 'frontend', got '%s'", p1.DisplayName())
	}

	p2 := Project{Name: "frontend", Label: "FE"}
	if p2.DisplayName() != "FE" {
		t.Errorf("expected 'FE', got '%s'", p2.DisplayName())
	}
}

func TestWriteConfigFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	cfg := &Config{
		Settings: Settings{
			Shell:       "/bin/zsh",
			Concurrency: 2,
			Exclude:     []string{"node_modules", "dist"},
		},
		GlobalAliases: map[string]string{"lint": "npm run lint"},
		Projects: []Project{
			{
				Name:    "frontend",
				Label:   "FE",
				Dir:     "src",
				Shell:   "/bin/bash",
				Env:     map[string]string{"PORT": "3000"},
				Aliases: map[string]string{"start": "yarn start"},
			},
		},
	}

	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	checks := []string{
		"[_settings]", "shell = /bin/zsh", "concurrency = 2", "exclude = node_modules, dist",
		"[*]", "lint = npm run lint",
		"[frontend]", "label = FE", "dir = src", "shell = /bin/bash",
		"env.PORT = 3000", "start = yarn start",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("missing %q in output:\n%s", check, content)
		}
	}
}

func TestConfigFullRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	original := &Config{
		Settings: Settings{
			Shell:       "/bin/zsh",
			Concurrency: 4,
			Exclude:     []string{"vendor"},
		},
		GlobalAliases: map[string]string{"test": "make test"},
		Projects: []Project{
			{
				Name:    "app",
				Label:   "APP",
				Dir:     "cmd",
				Env:     map[string]string{"GO111MODULE": "on"},
				Aliases: map[string]string{"build": "go build"},
			},
		},
	}

	if err := WriteConfig(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Settings.Shell != original.Settings.Shell {
		t.Errorf("settings shell: %q != %q", loaded.Settings.Shell, original.Settings.Shell)
	}
	if loaded.Settings.Concurrency != original.Settings.Concurrency {
		t.Errorf("settings concurrency: %d != %d", loaded.Settings.Concurrency, original.Settings.Concurrency)
	}
	if loaded.GlobalAliases["test"] != "make test" {
		t.Errorf("global alias: %q != %q", loaded.GlobalAliases["test"], "make test")
	}

	p := loaded.Projects[0]
	if p.Label != "APP" {
		t.Errorf("label: %q != %q", p.Label, "APP")
	}
	if p.Dir != "cmd" {
		t.Errorf("dir: %q != %q", p.Dir, "cmd")
	}
	if p.Env["GO111MODULE"] != "on" {
		t.Errorf("env: %q != %q", p.Env["GO111MODULE"], "on")
	}
	if p.Aliases["build"] != "go build" {
		t.Errorf("alias: %q != %q", p.Aliases["build"], "go build")
	}
}

func TestParseConfigColor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	content := "[_settings]\ncolor = blue\n\n[frontend]\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Settings.Color != "blue" {
		t.Errorf("expected color 'blue', got '%s'", cfg.Settings.Color)
	}
}

func TestResolveAccent(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"red", "\033[31m"},
		{"green", "\033[32m"},
		{"blue", "\033[34m"},
		{"cyan", "\033[36m"},
		{"", "\033[32m"},       // default green
		{"unknown", "\033[32m"}, // default green
		{"GREEN", "\033[32m"},   // case insensitive
	}

	for _, tt := range tests {
		got := resolveAccent(tt.name)
		if got != tt.expected {
			t.Errorf("resolveAccent(%q) = %q, want %q", tt.name, got, tt.expected)
		}
	}
}

func TestWriteConfigColor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	cfg := &Config{
		Settings:      Settings{Color: "magenta"},
		GlobalAliases: make(map[string]string),
		Projects: []Project{
			{Name: "app", Env: make(map[string]string), Aliases: make(map[string]string)},
		},
	}

	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "color = magenta") {
		t.Errorf("missing color in output:\n%s", string(data))
	}
}
