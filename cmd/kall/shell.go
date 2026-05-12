package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var errInterrupted = errors.New("interrupted")

// lineEditor is a minimal raw-mode line editor: history via Up/Down,
// cursor motion via Left/Right, tab completion against a candidate set,
// backspace, Ctrl+C, Ctrl+D (EOF on empty line). Single-byte ASCII only.
type lineEditor struct {
	prompt     string
	history    []string
	candidates []string
	pipedBuf   []byte // carries unread bytes across piped reads
}

type editState struct {
	buf      []rune
	cursor   int
	histIdx  int
	saved    []rune
	escState int // 0=normal, 1=ESC, 2=CSI ('[')
}

func (le *lineEditor) readLine() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return le.readLinePiped()
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return le.readLinePiped()
	}
	defer term.Restore(fd, oldState)

	st := &editState{histIdx: len(le.history)}
	le.render(st)

	pump := stdinPump()
	for chunk := range pump {
		for i := 0; i < len(chunk); i++ {
			b := chunk[i]

			if st.escState == 1 {
				if b == '[' {
					st.escState = 2
					continue
				}
				st.escState = 0
				continue
			}
			if st.escState == 2 {
				switch b {
				case 'A':
					le.historyPrev(st)
				case 'B':
					le.historyNext(st)
				case 'C':
					if st.cursor < len(st.buf) {
						st.cursor++
						le.render(st)
					}
				case 'D':
					if st.cursor > 0 {
						st.cursor--
						le.render(st)
					}
				case 'H':
					st.cursor = 0
					le.render(st)
				case 'F':
					st.cursor = len(st.buf)
					le.render(st)
				}
				st.escState = 0
				// Some terminals send ESC[3~ etc. Skip a trailing '~' or digits.
				for i+1 < len(chunk) && (chunk[i+1] == '~' || (chunk[i+1] >= '0' && chunk[i+1] <= '9') || chunk[i+1] == ';') {
					i++
				}
				continue
			}

			switch {
			case b == 0x1b:
				st.escState = 1
			case b == 0x03:
				fmt.Print("\r\n")
				return "", errInterrupted
			case b == 0x04:
				if len(st.buf) == 0 {
					fmt.Print("\r\n")
					return "", io.EOF
				}
			case b == '\r' || b == '\n':
				fmt.Print("\r\n")
				return string(st.buf), nil
			case b == 0x7f || b == 0x08:
				if st.cursor > 0 {
					st.buf = append(st.buf[:st.cursor-1], st.buf[st.cursor:]...)
					st.cursor--
					le.render(st)
				}
			case b == 0x09:
				le.complete(st)
			case b == 0x01: // Ctrl+A — home
				st.cursor = 0
				le.render(st)
			case b == 0x05: // Ctrl+E — end
				st.cursor = len(st.buf)
				le.render(st)
			case b == 0x0b: // Ctrl+K — kill to end
				st.buf = st.buf[:st.cursor]
				le.render(st)
			case b == 0x15: // Ctrl+U — kill line
				st.buf = st.buf[st.cursor:]
				st.cursor = 0
				le.render(st)
			case b == 0x17: // Ctrl+W — kill prev word
				le.killWord(st)
			case b == 0x0c: // Ctrl+L — clear screen
				fmt.Print("\033[H\033[2J")
				le.render(st)
			case b >= 0x20 && b < 0x7f:
				st.buf = append(st.buf[:st.cursor], append([]rune{rune(b)}, st.buf[st.cursor:]...)...)
				st.cursor++
				le.render(st)
			}
		}
	}
	return "", io.EOF
}

// readLinePiped is the cooked-mode fallback when stdin isn't a TTY.
func (le *lineEditor) readLinePiped() (string, error) {
	fmt.Print(le.prompt)
	pump := stdinPump()
	for {
		for i, b := range le.pipedBuf {
			if b == '\n' {
				line := string(le.pipedBuf[:i])
				le.pipedBuf = le.pipedBuf[i+1:]
				return strings.TrimRight(line, "\r"), nil
			}
		}
		chunk, ok := <-pump
		if !ok {
			if len(le.pipedBuf) > 0 {
				line := strings.TrimRight(string(le.pipedBuf), "\r")
				le.pipedBuf = nil
				return line, nil
			}
			return "", io.EOF
		}
		le.pipedBuf = append(le.pipedBuf, chunk...)
	}
}

func (le *lineEditor) render(st *editState) {
	fmt.Print("\r\033[K")
	fmt.Print(le.prompt)
	fmt.Print(string(st.buf))
	if st.cursor < len(st.buf) {
		fmt.Printf("\033[%dD", len(st.buf)-st.cursor)
	}
}

func (le *lineEditor) historyPrev(st *editState) {
	if st.histIdx <= 0 {
		return
	}
	if st.histIdx == len(le.history) {
		st.saved = append([]rune(nil), st.buf...)
	}
	st.histIdx--
	st.buf = []rune(le.history[st.histIdx])
	st.cursor = len(st.buf)
	le.render(st)
}

func (le *lineEditor) historyNext(st *editState) {
	if st.histIdx >= len(le.history) {
		return
	}
	st.histIdx++
	if st.histIdx == len(le.history) {
		st.buf = st.saved
		st.saved = nil
	} else {
		st.buf = []rune(le.history[st.histIdx])
	}
	st.cursor = len(st.buf)
	le.render(st)
}

func (le *lineEditor) killWord(st *editState) {
	if st.cursor == 0 {
		return
	}
	end := st.cursor
	// Skip trailing spaces
	for end > 0 && st.buf[end-1] == ' ' {
		end--
	}
	for end > 0 && st.buf[end-1] != ' ' {
		end--
	}
	st.buf = append(st.buf[:end], st.buf[st.cursor:]...)
	st.cursor = end
	le.render(st)
}

func (le *lineEditor) complete(st *editState) {
	wordStart := st.cursor
	for wordStart > 0 && st.buf[wordStart-1] != ' ' {
		wordStart--
	}
	prefix := string(st.buf[wordStart:st.cursor])

	// Only complete the first word (subcommand position).
	if wordStart != 0 {
		return
	}

	var matches []string
	for _, c := range le.candidates {
		if strings.HasPrefix(c, prefix) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		replacement := []rune(matches[0] + " ")
		tail := append([]rune(nil), st.buf[st.cursor:]...)
		st.buf = append(append([]rune(nil), st.buf[:wordStart]...), replacement...)
		st.buf = append(st.buf, tail...)
		st.cursor = wordStart + len(replacement)
		le.render(st)
		return
	}
	// Multiple matches: extend prefix to longest common prefix, then list.
	common := longestCommonPrefix(matches)
	if len(common) > len(prefix) {
		replacement := []rune(common)
		tail := append([]rune(nil), st.buf[st.cursor:]...)
		st.buf = append(append([]rune(nil), st.buf[:wordStart]...), replacement...)
		st.buf = append(st.buf, tail...)
		st.cursor = wordStart + len(replacement)
	}
	fmt.Print("\r\n")
	fmt.Println(strings.Join(matches, "  "))
	le.render(st)
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		j := 0
		for j < len(prefix) && j < len(s) && prefix[j] == s[j] {
			j++
		}
		prefix = prefix[:j]
		if prefix == "" {
			return ""
		}
	}
	return prefix
}
