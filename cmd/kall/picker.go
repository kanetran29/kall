package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// PickProjects presents an interactive TUI project selector with arrow keys
// and space to toggle. Falls back to simple numbered input if the terminal
// doesn't support raw mode (e.g., piped stdin, CI).
func PickProjects(available []string, currentlySelected []string) ([]string, error) {
	if len(available) == 0 {
		return nil, nil
	}

	// Try interactive mode first
	result, err := pickInteractive(available, currentlySelected)
	if err == errNoTTY {
		return pickSimple(available, currentlySelected)
	}
	return result, err
}

var errNoTTY = fmt.Errorf("not a tty")

func pickInteractive(available []string, currentlySelected []string) ([]string, error) {
	fd := int(os.Stdin.Fd())

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, errNoTTY
	}
	defer term.Restore(fd, oldState)

	// Initialize selection state
	selected := make([]bool, len(available))
	selSet := make(map[string]bool)
	for _, s := range currentlySelected {
		selSet[s] = true
	}
	for i, name := range available {
		selected[i] = selSet[name]
	}

	cursor := 0
	total := len(available)

	// Hide cursor
	fmt.Print("\033[?25l")

	// Restore cursor visibility on exit (covers normal + panic paths)
	defer func() {
		fmt.Print("\033[?25h")
		// Print newline so the shell prompt starts on a clean line
		fmt.Print("\r\n")
	}()

	draw := func(redraw bool) {
		if redraw {
			fmt.Printf("\033[%dA", total+2)
		}
		fmt.Print("Select projects (space: toggle, enter: confirm)\r\n\r\n")
		for i, name := range available {
			marker := "\u25c7 " // ◇
			if selected[i] {
				marker = "\u25c6 " // ◆
			}
			if i == cursor {
				fmt.Printf("\033[36m\u276f %s%s\033[0m\033[K\r\n", marker, name)
			} else {
				fmt.Printf("  %s%s\033[K\r\n", marker, name)
			}
		}
	}

	draw(false)

	// Read input: use a 3-byte buffer to capture escape sequences in one read.
	// A bare Esc key produces 1 byte; arrow keys produce 3 bytes (\x1b [ A/B).
	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		switch {
		case n == 1 && buf[0] == ' ':
			selected[cursor] = !selected[cursor]
			draw(true)

		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'):
			var result []string
			for i, name := range available {
				if selected[i] {
					result = append(result, name)
				}
			}
			return result, nil

		case n == 1 && (buf[0] == 'q' || buf[0] == 3):
			return nil, fmt.Errorf("cancelled")

		case n == 1 && buf[0] == '\x1b':
			// Bare Esc (no following bytes) — cancel
			return nil, fmt.Errorf("cancelled")

		case n == 3 && buf[0] == '\x1b' && buf[1] == '[':
			switch buf[2] {
			case 'A': // Up
				if cursor > 0 {
					cursor--
				}
				draw(true)
			case 'B': // Down
				if cursor < total-1 {
					cursor++
				}
				draw(true)
			}
		}
	}

	return nil, fmt.Errorf("unexpected input error")
}

// pickSimple is the fallback for non-interactive environments.
func pickSimple(available []string, currentlySelected []string) ([]string, error) {
	selSet := make(map[string]bool)
	for _, s := range currentlySelected {
		selSet[s] = true
	}

	fmt.Println("Available projects:")
	fmt.Println()
	for i, name := range available {
		marker := "  "
		if selSet[name] {
			marker = "* "
		}
		fmt.Printf("  %d. %s%s\n", i+1, marker, name)
	}
	fmt.Println()

	if len(currentlySelected) > 0 {
		fmt.Print("Enter project numbers (comma-separated, 'a' for all, enter to keep current): ")
	} else {
		fmt.Print("Enter project numbers (comma-separated, 'a' for all): ")
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	input = strings.TrimSpace(input)

	if input == "" {
		if len(currentlySelected) > 0 {
			return currentlySelected, nil
		}
		return nil, nil
	}

	if strings.EqualFold(input, "a") {
		return available, nil
	}

	var result []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		num, err := strconv.Atoi(part)
		if err != nil || num < 1 || num > len(available) {
			return nil, fmt.Errorf("invalid selection: %s (enter 1-%d)", part, len(available))
		}
		name := available[num-1]
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}

	return result, nil
}
