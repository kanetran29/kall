package main

import (
	"os"
	"sync"
)

// stdinChunks delivers raw byte chunks from os.Stdin to whichever consumer is
// currently active (shell line reader OR TUI key handler). A single
// process-wide goroutine pumps the fd so we never leak a reader across
// multiple TUI invocations within `kall shell`.
var (
	stdinChunks chan []byte
	stdinOnce   sync.Once
)

func stdinPump() chan []byte {
	stdinOnce.Do(func() {
		stdinChunks = make(chan []byte, 64)
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					close(stdinChunks)
					return
				}
				if n == 0 {
					continue
				}
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				stdinChunks <- chunk
			}
		}()
	})
	return stdinChunks
}
