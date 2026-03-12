package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRepos(t *testing.T) {
	dir := t.TempDir()

	// Create directories: two with .git, one without, one regular file
	os.MkdirAll(filepath.Join(dir, "project-a", ".git"), 0755)
	os.MkdirAll(filepath.Join(dir, "project-b", ".git"), 0755)
	os.MkdirAll(filepath.Join(dir, "not-a-repo"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0644)

	repos, err := DiscoverRepos(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}

	if repos[0] != "project-a" || repos[1] != "project-b" {
		t.Errorf("unexpected repos: %v", repos)
	}
}

func TestDiscoverReposEmpty(t *testing.T) {
	dir := t.TempDir()

	repos, err := DiscoverRepos(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

func TestFindRoot(t *testing.T) {
	dir := t.TempDir()

	// Resolve symlinks (macOS /var -> /private/var)
	dir, _ = filepath.EvalSymlinks(dir)

	// Create .kall in the root
	os.WriteFile(filepath.Join(dir, ".kall"), []byte("[test]\n"), 0644)

	// Create a nested subdirectory
	subdir := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(subdir, 0755)

	// Save and restore working directory
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	// Test from the root dir
	os.Chdir(dir)
	root, err := FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Errorf("expected root %q, got %q", dir, root)
	}

	// Test from a subdirectory — should walk up
	os.Chdir(subdir)
	root, err = FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Errorf("expected root %q from subdir, got %q", dir, root)
	}
}

func TestFindRootNotFound(t *testing.T) {
	dir := t.TempDir()

	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	os.Chdir(dir)
	_, err := FindRoot()
	if err == nil {
		t.Fatal("expected error when no .kall found")
	}
}
