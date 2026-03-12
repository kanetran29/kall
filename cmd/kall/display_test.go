package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderResultsSuccess(t *testing.T) {
	results := []Result{
		{Project: "frontend", Output: "Compiled successfully.\n", ExitCode: 0},
		{Project: "backend", Output: "Server started.\n", ExitCode: 0},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 50, false)
	out := buf.String()

	if !strings.Contains(out, "frontend") {
		t.Error("missing project name 'frontend'")
	}
	if !strings.Contains(out, "backend") {
		t.Error("missing project name 'backend'")
	}
	if !strings.Contains(out, "\u2713") {
		t.Error("missing success indicator")
	}
	if !strings.Contains(out, "Compiled successfully.") {
		t.Error("missing frontend output")
	}
	if !strings.Contains(out, "Server started.") {
		t.Error("missing backend output")
	}
	// Tab headers have horizontal rules
	if !strings.Contains(out, "\u2500") {
		t.Error("missing horizontal rule in tab header")
	}
}

func TestRenderResultsFailure(t *testing.T) {
	results := []Result{
		{Project: "broken", Output: "Error: build failed\n", ExitCode: 1},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 50, false)
	out := buf.String()

	if !strings.Contains(out, "\u2717") {
		t.Error("missing failure indicator")
	}
	if !strings.Contains(out, "Error: build failed") {
		t.Error("missing error output")
	}
}

func TestRenderResultsEmpty(t *testing.T) {
	var buf bytes.Buffer
	renderToWriter(&buf, []Result{}, 80, false)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty results, got %q", buf.String())
	}
}

func TestRenderResultsEmptyOutput(t *testing.T) {
	results := []Result{
		{Project: "quiet", Output: "", ExitCode: 0},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 50, false)
	out := buf.String()

	if !strings.Contains(out, "quiet") {
		t.Error("missing project name")
	}
	if !strings.Contains(out, "\u2713") {
		t.Error("missing success indicator")
	}
}

func TestRenderResultsVerbose(t *testing.T) {
	results := []Result{
		{Project: "frontend", Command: "yarn start", Output: "ready\n", ExitCode: 0},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 50, true)
	out := buf.String()

	if !strings.Contains(out, "$ yarn start") {
		t.Error("verbose mode should show the command")
	}
}

func TestRenderResultsNoVerbose(t *testing.T) {
	results := []Result{
		{Project: "frontend", Command: "yarn start", Output: "ready\n", ExitCode: 0},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 50, false)
	out := buf.String()

	if strings.Contains(out, "$ yarn start") {
		t.Error("non-verbose mode should not show the command")
	}
}

func TestRenderResultsSeparation(t *testing.T) {
	results := []Result{
		{Project: "a", Output: "one\n", ExitCode: 0},
		{Project: "b", Output: "two\n", ExitCode: 0},
	}

	var buf bytes.Buffer
	renderToWriter(&buf, results, 40, false)
	out := buf.String()

	// Projects should be separated by a blank line
	if !strings.Contains(out, "\n\n") {
		t.Error("expected blank line between projects")
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"\033[31mred\033[0m", "red"},
		{"\033[1m\033[36mbold cyan\033[0m", "bold cyan"},
		{"no codes here", "no codes here"},
	}

	for _, tt := range tests {
		got := stripAnsi(tt.input)
		if got != tt.expected {
			t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
