// Copyright (C) 2026  Faraz Fallahi <fffaraz@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fffaraz/dupdig/internal/tui"
)

var version = "dev" // version is set at build time via -ldflags "-X main.version=..."

// outputDirName is the directory created in the current working directory
// where all report files are written.
const outputDirName = "output"

type uiMode int

const (
	modeAuto uiMode = iota
	modeCLI
	modeTUI
)

func main() {
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Printf("dupdig %s\n", version)
		return
	}

	mode, args := parseArgs(args)
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [--tui|--cli] <source directory>\n", os.Args[0])
		os.Exit(1)
	}

	sourceDir := args[0]

	// Single argument mode: the reports are written to an "output" directory
	// created in the current working directory.
	outputDir := filepath.Join(".", outputDirName)

	// Run the terminal UI when attached to a TTY, unless overridden.
	useTUI := mode == modeTUI || (mode == modeAuto && isTerminal(os.Stdout))
	if useTUI {
		if err := tui.UI(sourceDir, outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "dupdig: %v\n", err)
		}
		return
	}

	os.Exit(runCLI(sourceDir, outputDir))
}

// parseArgs strips leading mode flags and returns the remaining positional
// arguments.
func parseArgs(args []string) (uiMode, []string) {
	mode := modeAuto
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tui", "-tui":
			mode = modeTUI
		case "--cli", "-cli":
			mode = modeCLI
		default:
			rest = append(rest, args[i:]...)
			return mode, rest
		}
	}
	return mode, rest
}

// isTerminal reports whether f is attached to a TTY (a character device).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
