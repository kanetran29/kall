package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FindRoot walks up from the current directory to find the nearest .kall file.
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".kall")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .kall config found. Run 'kall init' first")
		}
		dir = parent
	}
}

// DiscoverRepos finds subdirectories containing a .git directory or file.
func DiscoverRepos(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitPath := filepath.Join(root, e.Name(), ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, e.Name())
		}
	}

	sort.Strings(repos)
	return repos, nil
}
