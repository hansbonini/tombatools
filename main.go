/*
TombaTools - A collection of utilities for extracting and modifying game files from Tomba! (Ore no Tomba) for PlayStation.

Copyright © 2025 Hans Bonini
*/
package main

import (
	"fmt"
	"os"

	"github.com/hansbonini/tombatools/cmd"
)

// Version information (injected at build time)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Printf("TombaTools %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Go Version: %s\n", "go1.21")
		os.Exit(0)
	}

	cmd.Execute()
}
