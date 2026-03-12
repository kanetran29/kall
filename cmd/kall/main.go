package main

import "os"

// version is set at build time via ldflags
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
