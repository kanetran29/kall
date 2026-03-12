package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"
)

const (
	colorDim   = "\033[2m"
	colorCyan  = "\033[36m"
	colorBold  = "\033[1m"
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// RenderResults displays results as interactive tabs (TTY) or sequential output (pipe).
func RenderResults(results []Result, verbose bool, interactive bool) {
	if len(results) == 0 {
		return
	}

	width := termWidth()

	if interactive && term.IsTerminal(int(os.Stdout.Fd())) {
		renderTabs(results, verbose)
	} else {
		renderToWriter(os.Stdout, results, width, verbose)
	}
}

// renderTabs shows an interactive tab UI in the alternate screen buffer.
// Left/right arrows switch tabs. q, Esc, or Ctrl+C exits.
func renderTabs(results []Result, verbose bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		renderToWriter(os.Stdout, results, termWidth(), verbose)
		return
	}
	defer term.Restore(fd, oldState)

	// Alternate screen buffer + hide cursor
	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?25h\033[?1049l")

	active := 0

	draw := func() {
		fmt.Print("\033[H\033[J") // cursor home + clear
		width := termWidth()

		// Tab bar
		for i, r := range results {
			indicator := "\u2713"
			iColor := colorGreen
			if r.ExitCode != 0 {
				indicator = "\u2717"
				iColor = colorRed
			}

			if i == active {
				fmt.Printf(" %s\u25b8 %s%s %s%s%s",
					colorBold+colorCyan, r.Project, colorReset,
					iColor, indicator, colorReset)
			} else {
				fmt.Printf(" %s%s %s%s%s",
					colorDim, r.Project,
					iColor, indicator, colorReset)
			}

			if i < len(results)-1 {
				fmt.Printf(" %s\u2502%s", colorDim, colorReset)
			}
		}
		fmt.Print("\r\n")

		// Separator
		fmt.Printf("%s%s%s\r\n", colorDim, strings.Repeat("\u2500", width), colorReset)

		// Active tab content
		r := results[active]

		if verbose && r.Command != "" {
			fmt.Printf(" %s$ %s%s\r\n", colorDim, r.Command, colorReset)
		}

		errColor := ""
		if r.ExitCode != 0 {
			errColor = colorRed
		}

		output := strings.TrimRight(r.Output, "\n")
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				fmt.Printf(" %s%s%s\r\n", errColor, line, colorReset)
			}
		}

		// Help hint at bottom
		fmt.Printf("\r\n%s \u2190 \u2192 switch \u00b7 q quit%s", colorDim, colorReset)
	}

	draw()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		switch {
		case n == 1 && (buf[0] == 'q' || buf[0] == 3): // q, Ctrl+C
			return
		case n == 1 && buf[0] == '\x1b': // bare Esc
			return
		case n == 3 && buf[0] == '\x1b' && buf[1] == '[':
			switch buf[2] {
			case 'C': // Right
				if active < len(results)-1 {
					active++
					draw()
				}
			case 'D': // Left
				if active > 0 {
					active--
					draw()
				}
			}
		}
	}
}

// renderToWriter prints sequential output (for piped/non-TTY or tests).
func renderToWriter(w io.Writer, results []Result, width int, verbose bool) {
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}

		indicator := "\u2713"
		iColor := colorGreen
		if r.ExitCode != 0 {
			indicator = "\u2717"
			iColor = colorRed
		}

		prefixLen := len(r.Project) + 4
		ruleLen := width - prefixLen
		if ruleLen < 2 {
			ruleLen = 2
		}
		rule := strings.Repeat("\u2500", ruleLen)

		fmt.Fprintf(w, " %s%s%s %s%s%s %s%s%s\n",
			colorBold+colorCyan, r.Project, colorReset,
			iColor, indicator, colorReset,
			colorDim, rule, colorReset,
		)

		if verbose && r.Command != "" {
			fmt.Fprintf(w, " %s$ %s%s\n", colorDim, r.Command, colorReset)
		}

		errColor := ""
		if r.ExitCode != 0 {
			errColor = colorRed
		}

		output := strings.TrimRight(r.Output, "\n")
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				fmt.Fprintf(w, " %s%s%s\n", errColor, line, colorReset)
			}
		}
	}
}
