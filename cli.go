package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fffaraz/dupdig/internal/scan"
)

// runCLI performs a scan in the plain non-interactive mode, mirroring the
// original command-line output. It returns the process exit code.
func runCLI(sourceDir, outputDir string) int {
	log := func(format string, args ...interface{}) {
		fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
	}

	_, err := scan.Run(scan.Options{
		SourceDir: sourceDir,
		OutputDir: outputDir,
		Log:       log,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		var scanErr *scan.ScanError
		if errors.As(err, &scanErr) && scanErr.Walk {
			return 2 // walking the tree failed: historical exit code
		}
		return 1
	}

	fmt.Printf("%s Done!\n", time.Now().Format("2006-01-02 15:04:05"))
	return 0
}
