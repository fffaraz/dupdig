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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ignoredDirs = map[string]bool{
	".cache":  true,
	".config": true,
	".git":    true,
	".local":  true,
}

var systemDirs = map[string]bool{
	"/dev":  true,
	"/proc": true,
	"/sys":  true,
	"/run":  true,
}

type fileInfo struct {
	hash string // hex string of sha256 hash
	size int64  // file size in bytes
	path string // relative path from source directory
}

type dupGroup struct {
	hash  string   // hash of the duplicate files
	size  int64    // size of each duplicate file
	count int      // number of duplicate files
	waste int64    // total wasted space in bytes (size * (count - 1))
	paths []string // sorted list of relative paths of duplicate files
}

var dirHash = strings.Repeat("0", sha256.Size*2) // hash for directories (all zeros)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <source directory> <output directory>\n", os.Args[0])
		os.Exit(1)
	}

	sourceDir := os.Args[1]
	outputDir := os.Args[2]

	// create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// create output files errors.txt
	errorsFile, err := os.Create(filepath.Join(outputDir, "errors.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating errors.txt: %v\n", err)
		os.Exit(1)
	}
	defer errorsFile.Close()

	// create empty-dirs.txt output file
	emptyDirsFile, err := os.Create(filepath.Join(outputDir, "empty-dirs.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating empty-dirs.txt: %v\n", err)
		os.Exit(1)
	}
	defer emptyDirsFile.Close()

	// create empty-files.txt output file
	emptyFilesFile, err := os.Create(filepath.Join(outputDir, "empty-files.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating empty-files.txt: %v\n", err)
		os.Exit(1)
	}
	defer emptyFilesFile.Close()

	// create duplicates.txt output file
	duplicatesFile, err := os.Create(filepath.Join(outputDir, "duplicates.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating duplicates.txt: %v\n", err)
		os.Exit(1)
	}
	defer duplicatesFile.Close()

	// create files.txt output file
	filesFile, err := os.Create(filepath.Join(outputDir, "files.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating files.txt: %v\n", err)
		os.Exit(1)
	}
	defer filesFile.Close()

	// create rm-duplicates.sh output file
	rmDuplicatesFile, err := os.Create(filepath.Join(outputDir, "rm-duplicates.sh"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating rm-duplicates.sh: %v\n", err)
		os.Exit(1)
	}
	defer rmDuplicatesFile.Close()
	fmt.Fprintf(rmDuplicatesFile, "#!/bin/bash\n\n# This script deletes duplicate files listed in duplicates.txt\n# Review the file before running this script!\n\n")

	var files []fileInfo
	sourcePrefix := filepath.Clean(sourceDir) + string(filepath.Separator)
	counter := 0

	fmt.Printf("%s Starting scan of %s...\n", time.Now().Format("2006-01-02 15:04:05"), sourceDir)
	startTime := time.Now()

	errWalk := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(errorsFile, "skipping: %v\n", err)
			if info != nil && info.IsDir() {
				return filepath.SkipDir // skip entire directory if we can't access it
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(errorsFile, "skipping symlink: %s\n", path)
			return nil
		}

		if info.IsDir() {
			absPath, _ := filepath.Abs(path)
			if systemDirs[absPath] {
				fmt.Fprintf(errorsFile, "skipping system directory: %s\n", path)
				return filepath.SkipDir // skip system directories
			}
			if ignoredDirs[info.Name()] {
				fmt.Fprintf(errorsFile, "skipping ignored directory: %s\n", path)
				return filepath.SkipDir // skip ignored directories
			}
			if path == sourceDir {
				return nil // skip the root directory itself
			}
			files = append(files, fileInfo{
				hash: dirHash,
				size: 0,
				path: strings.TrimPrefix(path, sourcePrefix),
			})
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(errorsFile, "skipping file: %v\n", err)
			return nil
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			fmt.Fprintf(errorsFile, "read error: %v\n", err)
			return nil
		}

		files = append(files, fileInfo{
			hash: fmt.Sprintf("%x", h.Sum(nil)),
			size: info.Size(),
			path: strings.TrimPrefix(path, sourcePrefix),
		})

		counter++
		if counter%100 == 0 {
			fmt.Printf("%s %d files hashed...\n", time.Now().Format("2006-01-02 15:04:05"), counter)
		}
		return nil
	})
	if errWalk != nil {
		fmt.Fprintf(errorsFile, "error: %v\n", errWalk)
		os.Exit(2)
	}

	elapsed := time.Since(startTime)
	hours := int(elapsed.Hours())
	mins := int(elapsed.Minutes()) - hours*60
	secs := elapsed.Seconds() - float64(hours*3600+mins*60)
	fmt.Printf("%s Hashed %d files in %dh%dm%.2fs\n", time.Now().Format("2006-01-02 15:04:05"), counter, hours, mins, secs)

	// Find duplicates (group files by hash, ignoring directories)
	hashGroups := make(map[string][]fileInfo)
	for _, f := range files {
		if f.size == 0 && f.hash == dirHash {
			continue // skip directories
		}
		hashGroups[f.hash] = append(hashGroups[f.hash], f)
	}

	// Collect duplicate groups (2+ files with same hash)
	var dups []dupGroup
	for hash, group := range hashGroups {
		if len(group) < 2 {
			continue // not a duplicate if only one file with this hash
		}
		size := group[0].size
		count := len(group)
		waste := size * int64(count-1) // total wasted space in bytes
		var paths []string
		for _, f := range group {
			if f.size != size {
				fmt.Fprintf(errorsFile, "hash collision: %s has different sizes (%d vs %d)\n", hash, size, f.size)
			}
			paths = append(paths, f.path)
		}
		sort.Strings(paths)
		dups = append(dups, dupGroup{
			hash:  hash,
			size:  size,
			count: count,
			waste: waste,
			paths: paths,
		})
	}

	// Sort by wasted space descending
	sort.Slice(dups, func(i, j int) bool {
		return dups[i].waste > dups[j].waste
	})

	// Print duplicate summary
	if len(dups) > 0 {
		var totalWaste int64
		for _, d := range dups {
			totalWaste += d.waste
		}
		fmt.Fprintf(duplicatesFile, "=== %d Duplicate Files (%.2f MB wasted) ===\n\n", len(dups), float64(totalWaste)/(1024*1024))
		for _, d := range dups {
			sizeMB := float64(d.size) / (1024 * 1024)
			fmt.Fprintf(duplicatesFile, "%.2f MB = %d x %.2f MB\t%s\n", sizeMB*float64(d.count), d.count, sizeMB, d.hash)
			first := true
			for _, p := range d.paths {
				fmt.Fprintf(duplicatesFile, "\t%s\n", p)
				if first {
					fmt.Fprintf(rmDuplicatesFile, "# Keep: \"%s\" %s\n", p, d.hash)
					first = false
				} else {
					fmt.Fprintf(rmDuplicatesFile, "rm \"%s\"\n", p)
				}
			}
			fmt.Fprintln(duplicatesFile)
			fmt.Fprintln(rmDuplicatesFile)
		}
	}

	// Collect stats
	var numFiles, numDirs, numEmpty int
	for _, f := range files {
		if f.size == 0 && f.hash == dirHash {
			numDirs++
		} else {
			numFiles++
			if f.size == 0 {
				numEmpty++
			}
		}
	}

	// Print full sorted file list
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	// Find empty directories (directories with no children in the file list)
	dirSet := make(map[string]bool)
	for _, f := range files {
		if f.hash == dirHash {
			dirSet[f.path] = true
		}
	}
	// A directory is empty if no other entry has it as a parent
	hasChildren := make(map[string]bool)
	for _, f := range files {
		parent := filepath.Dir(f.path)
		for parent != "." && parent != "" {
			hasChildren[parent] = true
			parent = filepath.Dir(parent)
		}
	}
	var emptyDirs []string
	for dir := range dirSet {
		if !hasChildren[dir] {
			emptyDirs = append(emptyDirs, dir)
		}
	}
	sort.Strings(emptyDirs)

	fmt.Fprintf(emptyDirsFile, "=== %d empty directories ===\n\n", len(emptyDirs))
	for _, d := range emptyDirs {
		fmt.Fprintf(emptyDirsFile, "%s\n", d)
	}

	fmt.Fprintf(filesFile, "=== %d files, %d directories, %d empty files, %d empty directories ===\n", numFiles, numDirs, numEmpty, len(emptyDirs))
	fmt.Fprintf(emptyFilesFile, "=== %d empty files === e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n\n", numEmpty)
	for _, f := range files {
		fmt.Fprintf(filesFile, "%s\t%d\t%s\n", f.hash, f.size, f.path)
		if f.size == 0 && f.hash != dirHash {
			fmt.Fprintf(emptyFilesFile, "%s\n", f.path)
		}
	}
}
