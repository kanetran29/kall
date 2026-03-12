package main

import (
	"os"
	"path/filepath"
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
		Projects: []Project{
			{Name: "frontend", Aliases: map[string]string{"start": "yarn start"}},
			{Name: "backend", Aliases: map[string]string{"start": "flask run", "test": "pytest"}},
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
	if !contains(content, "[frontend]") || !contains(content, "[backend]") {
		t.Errorf("missing section headers in output:\n%s", content)
	}
	if !contains(content, "start = yarn start") {
		t.Errorf("missing frontend alias in output:\n%s", content)
	}
	if !contains(content, "start = flask run") {
		t.Errorf("missing backend alias in output:\n%s", content)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".kall")

	original := &Config{
		Projects: []Project{
			{Name: "app", Aliases: map[string]string{"build": "make build", "test": "make test"}},
			{Name: "lib", Aliases: map[string]string{}},
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
