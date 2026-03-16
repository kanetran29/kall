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

func highlightMatches(line, query string) string {
	if query == "" {
		return line
	}
	plain := stripAnsi(line)
	lowerPlain := strings.ToLower(plain)
	lowerQuery := strings.ToLower(query)

	// Find match positions in plain text
	type span struct{ start, end int }
	var spans []span
	off := 0
	for {
		idx := strings.Index(lowerPlain[off:], lowerQuery)
		if idx < 0 {
			break
		}
		spans = append(spans, span{off + idx, off + idx + len(query)})
		off += idx + len(query)
	}
	if len(spans) == 0 {
		return line
	}

	// Build mapping from plain-text index to original string index
	mapping := make([]int, len(plain)+1)
	pi := 0
	for oi := 0; oi < len(line); oi++ {
		if pi <= len(plain) {
			mapping[pi] = oi
		}
		// Skip ANSI sequences
		if line[oi] == '\x1b' && oi+1 < len(line) && line[oi+1] == '[' {
			for oi < len(line) && line[oi] != 'm' {
				oi++
			}
			continue
		}
		pi++
	}
	if pi <= len(plain) {
		mapping[pi] = len(line)
	}

	// Reconstruct line with reverse-video highlights
	var result strings.Builder
	prev := 0
	for _, s := range spans {
		origStart := mapping[s.start]
		origEnd := mapping[s.end]
		result.WriteString(line[prev:origStart])
		result.WriteString("\033[7m")
		result.WriteString(line[origStart:origEnd])
		result.WriteString("\033[27m")
		prev = origEnd
	}
	result.WriteString(line[prev:])
	return result.String()
}

func rebuildMatches(lines []string, query string) []int {
	if query == "" {
		return nil
	}
	lowerQuery := strings.ToLower(query)
	var matches []int
	for i, line := range lines {
		if strings.Contains(strings.ToLower(stripAnsi(line)), lowerQuery) {
			matches = append(matches, i)
		}
	}
	return matches
}

func jumpToMatch(allLines []string, matches []int, idx int, scrollOffsets []int, tab int, height int, expanded bool, verbose bool, command string) {
	if len(matches) == 0 {
		return
	}
	targetLine := matches[idx]
	totalLines := len(allLines)

	overhead := 3
	if !expanded {
		overhead++
	}
	if verbose && command != "" {
		overhead++
	}
	maxLines := height - overhead
	if maxLines < 1 {
		maxLines = 1
	}

	if totalLines <= maxLines {
		scrollOffsets[tab] = 0
		return
	}

	// scrollOffset is measured from the bottom: offset=0 means viewing the last maxLines lines
	// Visible window: lines[totalLines - maxLines - offset : totalLines - offset]
	// We want targetLine centered in viewport
	center := maxLines / 2
	// start = totalLines - maxLines - offset, we want start = targetLine - center
	desiredStart := targetLine - center
	if desiredStart < 0 {
		desiredStart = 0
	}
	maxOffset := totalLines - maxLines
	offset := totalLines - maxLines - desiredStart
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	scrollOffsets[tab] = offset
}

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
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
	scrollOffsets := make([]int, total)
	expanded := false

	// Search state
	searchMode := false
	searchInput := ""
	searchQuery := make([]string, total)
	matchLines := make([][]int, total)
	matchIdx := make([]int, total)

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

		hasOverflow := false
		b.WriteString("\033[H") // cursor home (no clear — overwrite in place)

		lp := lives[active]

		if expanded {
			// Expanded: minimal header — just active project name + dot
			var dotColor string
			if !lp.Done {
				dotColor = colorYellow
			} else if lp.ExitCode == 0 {
				dotColor = accent
			} else {
				dotColor = colorRed
			}
			elapsed := formatDuration(lp.Elapsed())
			elapsedStr := ""
			if elapsed != "" {
				elapsedStr = fmt.Sprintf(" %s%s%s", colorDim, elapsed, colorReset)
			}
			fmt.Fprintf(&b, " %s%s%s %s\u25cf%s%s\033[K\r\n",
				colorBold, lp.Project, colorReset,
				dotColor, colorReset, elapsedStr)
		} else {
			// Normal: tab bar + separator
			for i, lp := range lives {
				var dotColor string
				if !lp.Done {
					dotColor = colorYellow
				} else if lp.ExitCode == 0 {
					dotColor = accent
				} else {
					dotColor = colorRed
				}

				elapsed := formatDuration(lp.Elapsed())
				elapsedStr := ""
				if elapsed != "" {
					elapsedStr = fmt.Sprintf(" %s%s%s", colorDim, elapsed, colorReset)
				}

				if i == active {
					fmt.Fprintf(&b, " %s%d:%s%s %s\u25cf%s%s",
						colorBold, i+1, lp.Project, colorReset,
						dotColor, colorReset, elapsedStr)
				} else {
					fmt.Fprintf(&b, " %s%d:%s%s %s\u25cf%s%s",
						colorDim, i+1, lp.Project, colorReset,
						dotColor, colorReset, elapsedStr)
				}

				if i < total-1 {
					b.WriteString("  ")
				}
			}
			b.WriteString("\033[K\r\n") // clear rest of line, newline

			// Separator
			fmt.Fprintf(&b, "%s%s%s\033[K\r\n", colorDim, strings.Repeat("\u2500", width), colorReset)
		}

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
			// Reserve lines for: header(1) + separator(0-1) + verbose(0-1) + hint(2)
			overhead := 3
			if !expanded {
				overhead++ // separator line
			}
			if verbose && lp.Command != "" {
				overhead++
			}
			maxLines := height - overhead
			if maxLines < 1 {
				maxLines = 1
			}
			if len(lines) > maxLines {
				hasOverflow = true
				maxOffset := len(lines) - maxLines
				if scrollOffsets[active] > maxOffset {
					scrollOffsets[active] = maxOffset
				}
				start := len(lines) - maxLines - scrollOffsets[active]
				lines = lines[start : start+maxLines]
			} else {
				scrollOffsets[active] = 0
			}
			for _, line := range lines {
				if searchQuery[active] != "" {
					line = highlightMatches(line, searchQuery[active])
				}
				fmt.Fprintf(&b, " %s%s%s\033[K\r\n", errColor, line, colorReset)
			}
		} else if !lp.Done {
			fmt.Fprintf(&b, " %s%sWaiting for output...%s\033[K\r\n", colorYellow, colorDim, colorReset)
		}

		b.WriteString("\033[J") // clear from cursor to end of screen (remove stale lines from previous tab)

		// Help hint — left keys, right-aligned status
		doneCount := countDone()

		if searchMode {
			// Search prompt
			prompt := fmt.Sprintf(" /%s\u2588", searchInput)
			pad := width - len(stripAnsi(prompt))
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(&b, "\r\n%s%s%s%s", prompt, colorDim, strings.Repeat(" ", pad), colorReset)
		} else {
			var left []string
			left = append(left, "\u2190 \u2192 switch")
			if hasOverflow {
				left = append(left, "\u2191 \u2193 scroll")
			}
			if expanded {
				left = append(left, "^O collapse")
			} else {
				left = append(left, "^O expand")
			}
			if lp.IsDone() {
				left = append(left, "r rerun")
			} else {
				left = append(left, "x kill")
			}
			if doneCount > 0 {
				left = append(left, "R all")
			}
			if doneCount < total {
				left = append(left, "X all")
			}
			left = append(left, "/ search")
			left = append(left, "q quit")
			leftStr := strings.Join(left, "   ")

			var right string
			if searchQuery[active] != "" {
				m := len(matchLines[active])
				if m > 0 {
					right = fmt.Sprintf("%d/%d matches", matchIdx[active]+1, m)
				} else {
					right = "no matches"
				}
			} else if doneCount < total {
				right = fmt.Sprintf("%d/%d done", doneCount, total)
			}

			padding := width - len(stripAnsi(leftStr)) - len(right) - 2 // 2 for leading space + trailing space
			if padding < 1 {
				padding = 1
			}
			fmt.Fprintf(&b, "\r\n%s %s%s%s%s", colorDim, leftStr, strings.Repeat(" ", padding), right, colorReset)
		}

		// Single write — no flicker
		fmt.Print(b.String())
	}

	draw()

	// Stdin reader goroutine
	keyCh := make(chan []byte, 10)
	go func() {
		buf := make([]byte, 6)
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

			// Search mode input handling
			if searchMode {
				switch {
				case n == 1 && key[0] == '\r': // Enter — confirm search
					searchQuery[active] = searchInput
					output := strings.TrimRight(lives[active].Output(), "\n")
					if output != "" {
						allLines := strings.Split(output, "\n")
						matchLines[active] = rebuildMatches(allLines, searchInput)
						if len(matchLines[active]) > 0 {
							matchIdx[active] = 0
							// Jump to first match
							jumpToMatch(allLines, matchLines[active], matchIdx[active], scrollOffsets, active, termHeight(), expanded, verbose, lives[active].Command)
						}
					}
					searchMode = false
					draw()
				case n == 1 && key[0] == '\x1b': // Esc — cancel search input
					searchMode = false
					draw()
				case n == 1 && key[0] == 127: // Backspace
					if len(searchInput) > 0 {
						searchInput = searchInput[:len(searchInput)-1]
					}
					draw()
				case n == 1 && key[0] == 3: // Ctrl+C — cancel
					searchMode = false
					draw()
				case n == 1 && key[0] >= 0x20 && key[0] <= 0x7e: // Printable ASCII
					searchInput += string(key[0])
					draw()
				}
				continue
			}

			switch {
			case n == 1 && (key[0] == 'q' || key[0] == 3): // q, Ctrl+C
				return liveToResults(lives)
			case n == 1 && key[0] == '\x1b': // bare Esc — clear search
				if searchQuery[active] != "" {
					searchQuery[active] = ""
					matchLines[active] = nil
					matchIdx[active] = 0
					draw()
				}
			case n == 1 && key[0] == '/': // enter search mode
				searchMode = true
				searchInput = ""
				draw()
			case n == 1 && key[0] == 'n': // next match
				if len(matchLines[active]) > 0 {
					matchIdx[active] = (matchIdx[active] + 1) % len(matchLines[active])
					output := strings.TrimRight(lives[active].Output(), "\n")
					if output != "" {
						allLines := strings.Split(output, "\n")
						jumpToMatch(allLines, matchLines[active], matchIdx[active], scrollOffsets, active, termHeight(), expanded, verbose, lives[active].Command)
					}
					draw()
				}
			case n == 1 && key[0] == 'N': // previous match
				if len(matchLines[active]) > 0 {
					matchIdx[active] = (matchIdx[active] - 1 + len(matchLines[active])) % len(matchLines[active])
					output := strings.TrimRight(lives[active].Output(), "\n")
					if output != "" {
						allLines := strings.Split(output, "\n")
						jumpToMatch(allLines, matchLines[active], matchIdx[active], scrollOffsets, active, termHeight(), expanded, verbose, lives[active].Command)
					}
					draw()
				}
			case n == 1 && key[0] == 'r': // rerun active tab
				scrollOffsets[active] = 0
				searchQuery[active] = ""
				matchLines[active] = nil
				matchIdx[active] = 0
				lives[active].launch(doneCh, active)
				draw()
			case n == 1 && key[0] == 0x0F: // Ctrl+O toggle expand/collapse
				expanded = !expanded
				draw()
			case n == 1 && key[0] == 'x': // kill active tab
				lives[active].Kill()
				draw()
			case n == 1 && key[0] == 'R': // rerun all done tabs
				for i, lp := range lives {
					if lp.IsDone() {
						scrollOffsets[i] = 0
						searchQuery[i] = ""
						matchLines[i] = nil
						matchIdx[i] = 0
						lp.launch(doneCh, i)
					}
				}
				draw()
			case n == 1 && key[0] == 'X': // kill all running tabs
				for _, lp := range lives {
					lp.Kill()
				}
				draw()
			case n == 1 && key[0] == 'k': // vim scroll up
				scrollOffsets[active]++
				draw()
			case n == 1 && key[0] == 'j': // vim scroll down
				if scrollOffsets[active] > 0 {
					scrollOffsets[active]--
					draw()
				}
			case n == 1 && key[0] == 'g': // jump to top
				scrollOffsets[active] = 999999
				draw()
			case n == 1 && key[0] == 'G': // jump to bottom (follow mode)
				scrollOffsets[active] = 0
				draw()
			case n == 1 && key[0] >= '1' && key[0] <= '9': // tab number shortcuts
				idx := int(key[0] - '1')
				if idx >= total {
					idx = total - 1
				}
				active = idx
				draw()
			case n == 3 && key[0] == '\x1b' && key[1] == '[':
				switch key[2] {
				case 'C': // Right
					active = (active + 1) % total
					draw()
				case 'D': // Left
					active = (active - 1 + total) % total
					draw()
				case 'A': // Up
					scrollOffsets[active]++
					draw()
				case 'B': // Down
					if scrollOffsets[active] > 0 {
						scrollOffsets[active]--
						draw()
					}
				}
			case n == 4 && key[0] == '\x1b' && key[1] == '[' && key[3] == '~':
				pageSize := termHeight() / 2
				if pageSize < 1 {
					pageSize = 1
				}
				switch key[2] {
				case '5': // Page Up
					scrollOffsets[active] += pageSize
					draw()
				case '6': // Page Down
					scrollOffsets[active] -= pageSize
					if scrollOffsets[active] < 0 {
						scrollOffsets[active] = 0
					}
					draw()
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
