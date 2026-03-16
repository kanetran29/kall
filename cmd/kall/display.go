package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	colorDim  = "\033[2m"
	colorBold = "\033[1m"
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
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

func termHeight() int {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

// RenderLive shows the tab UI immediately with live-streaming output.
// Returns final results once every command has finished or the user quits.
func RenderLive(lives []*LiveProject, doneCh chan int, verbose bool, accent string) []Result {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: wait for all to finish, render sequentially
		remaining := len(lives)
		for remaining > 0 {
			<-doneCh
			remaining--
		}
		return liveToResults(lives)
	}
	defer term.Restore(fd, oldState)

	// Alternate screen buffer + hide cursor
	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?25h\033[?1049l")

	active := 0
	total := len(lives)

	countDone := func() int {
		n := 0
		for _, lp := range lives {
			if lp.IsDone() {
				n++
			}
		}
		return n
	}

	draw := func() {
		var b strings.Builder
		width := termWidth()
		height := termHeight()

		b.WriteString("\033[H") // cursor home (no clear — overwrite in place)

		// Tab bar
		for i, lp := range lives {
			var dotColor string
			if !lp.Done {
				dotColor = colorYellow
			} else if lp.ExitCode == 0 {
				dotColor = accent
			} else {
				dotColor = colorRed
			}

			if i == active {
				fmt.Fprintf(&b, " %s%s%s %s\u25cf%s",
					colorBold, lp.Project, colorReset,
					dotColor, colorReset)
			} else {
				fmt.Fprintf(&b, " %s%s%s %s\u25cf%s",
					colorDim, lp.Project, colorReset,
					dotColor, colorReset)
			}

			if i < total-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\033[K\r\n") // clear rest of line, newline

		// Separator
		fmt.Fprintf(&b, "%s%s%s\033[K\r\n", colorDim, strings.Repeat("\u2500", width), colorReset)

		// Active tab content
		lp := lives[active]

		if verbose && lp.Command != "" {
			fmt.Fprintf(&b, " %s$ %s%s\033[K\r\n", accent, lp.Command, colorReset)
		}

		errColor := ""
		if lp.Done && lp.ExitCode != 0 {
			errColor = colorRed
		}

		output := strings.TrimRight(lp.Output(), "\n")
		if output != "" {
			lines := strings.Split(output, "\n")
			// Reserve lines for: tab bar(1) + separator(1) + verbose(0-1) + hint(2)
			overhead := 4
			if verbose && lp.Command != "" {
				overhead++
			}
			maxLines := height - overhead
			if maxLines < 1 {
				maxLines = 1
			}
			// Show the tail if output exceeds screen
			if len(lines) > maxLines {
				lines = lines[len(lines)-maxLines:]
			}
			for _, line := range lines {
				fmt.Fprintf(&b, " %s%s%s\033[K\r\n", errColor, line, colorReset)
			}
		} else if !lp.Done {
			fmt.Fprintf(&b, " %s%sWaiting for output...%s\033[K\r\n", colorYellow, colorDim, colorReset)
		}

		b.WriteString("\033[J") // clear from cursor to end of screen (remove stale lines from previous tab)

		// Help hint — left keys, right-aligned status
		doneCount := countDone()
		var left []string
		left = append(left, "\u2190 \u2192 switch")
		if lp.IsDone() {
			left = append(left, "r rerun")
		} else {
			left = append(left, "x kill")
		}
		left = append(left, "q quit")
		leftStr := strings.Join(left, "   ")

		var right string
		if doneCount < total {
			right = fmt.Sprintf("%d/%d done", doneCount, total)
		}

		padding := width - len(stripAnsi(leftStr)) - len(right) - 2 // 2 for leading space + trailing space
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(&b, "\r\n%s %s%s%s%s", colorDim, leftStr, strings.Repeat(" ", padding), right, colorReset)

		// Single write — no flicker
		fmt.Print(b.String())
	}

	draw()

	// Stdin reader goroutine
	keyCh := make(chan []byte, 10)
	go func() {
		buf := make([]byte, 3)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(keyCh)
				return
			}
			b := make([]byte, n)
			copy(b, buf[:n])
			keyCh <- b
		}
	}()

	// Ticker for live output refresh
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-doneCh:
			draw()
			if countDone() == total {
				return liveToResults(lives)
			}

		case <-ticker.C:
			if countDone() < total {
				draw()
			}

		case key, ok := <-keyCh:
			if !ok {
				return liveToResults(lives)
			}
			n := len(key)

			switch {
			case n == 1 && (key[0] == 'q' || key[0] == 3): // q, Ctrl+C
				return liveToResults(lives)
			case n == 1 && key[0] == '\x1b': // bare Esc
				return liveToResults(lives)
			case n == 1 && key[0] == 'r': // rerun active tab
				lives[active].launch(doneCh, active)
				draw()
			case n == 1 && key[0] == 'x': // kill active tab
				lives[active].Kill()
				draw()
			case n == 3 && key[0] == '\x1b' && key[1] == '[':
				switch key[2] {
				case 'C': // Right
					if active < total-1 {
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
}

// liveToResults converts LiveProject handles to final Result values.
func liveToResults(lives []*LiveProject) []Result {
	results := make([]Result, len(lives))
	for i, lp := range lives {
		results[i] = Result{
			Project:  lp.Project,
			Command:  lp.Command,
			Output:   lp.Output(),
			ExitCode: lp.ExitCode,
		}
	}
	return results
}

// RenderSequential displays results as sequential output (for piped/non-TTY).
func RenderSequential(results []Result, verbose bool, accent string) {
	renderToWriter(os.Stdout, results, termWidth(), verbose, accent)
}

// renderToWriter prints sequential output to a writer (for piped/non-TTY or tests).
func renderToWriter(w io.Writer, results []Result, width int, verbose bool, accent string) {
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}

		var dotColor string
		if r.ExitCode == 0 {
			dotColor = accent
		} else {
			dotColor = colorRed
		}

		prefixLen := len(r.Project) + 5 // name + space + dot + space + space
		ruleLen := width - prefixLen
		if ruleLen < 2 {
			ruleLen = 2
		}
		rule := strings.Repeat("\u2500", ruleLen)

		fmt.Fprintf(w, " %s%s%s %s\u25cf%s %s%s%s\n",
			colorBold, r.Project, colorReset,
			dotColor, colorReset,
			colorDim, rule, colorReset,
		)

		if verbose && r.Command != "" {
			fmt.Fprintf(w, " %s$ %s%s\n", accent, r.Command, colorReset)
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
