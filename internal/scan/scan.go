// Package scan walks a directory tree, hashes every file with SHA-256
// and identifies duplicate files, empty files and empty directories.
//
// The scanning logic is sequential by design: parallel reads thrash the
// seek heads of spinning disks, making concurrent hashing slower.
package scan

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ignoredDirs contains directory names that are always skipped.
var ignoredDirs = map[string]bool{
	".cache":  true,
	".config": true,
	".git":    true,
	".local":  true,
}

// systemDirs contains absolute paths that are always skipped.
var systemDirs = map[string]bool{
	"/dev":  true,
	"/proc": true,
	"/run":  true,
	"/sys":  true,
}

// dirHash tags directory entries (an all-zero SHA-256 hash).
var dirHash = strings.Repeat("0", sha256.Size*2)

// emptyHash is the SHA-256 of an empty file, reused in the report header.
const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// FileInfo describes one scanned directory entry.
type FileInfo struct {
	Hash string // hex SHA-256; dirHash for directories
	Size int64  // file size in bytes
	Path string // slash-separated path relative to the source directory
}

// DupGroup groups the files that share the same content hash.
type DupGroup struct {
	Hash  string   // shared content hash
	Size  int64    // size of each member (bytes)
	Count int      // number of duplicate files
	Waste int64    // wasted bytes: Size * (Count - 1)
	Paths []string // sorted relative paths
}

// Stats summarizes a finished scan.
type Stats struct {
	TotalFiles      int
	TotalDirs       int
	EmptyFiles      int
	EmptyDirs       int
	DuplicateGroups int
	TotalWaste      int64
	Elapsed         time.Duration
}

// Result contains everything a scan discovered.
type Result struct {
	Files      []FileInfo
	DupGroups  []DupGroup
	EmptyFiles []string
	EmptyDirs  []string
	Errors     []string
	Stats      Stats
}

// Progress describes the current state of an in-flight scan.
type Progress struct {
	Hashed      int    // entries recorded so far (files and directories)
	BytesHashed int64  // bytes hashed so far
	CurrentPath string // file most recently hashed
	LastError   string // most recent traversal error, if any
	ErrorCount  int
	Elapsed     time.Duration
}

// Options configures a scan.
type Options struct {
	SourceDir string
	OutputDir string

	// Ignore lists directories or files to skip. Each pattern may be a
	// directory or file name (matched against any entry of that name) or a
	// slash-separated relative path (matched against that exact path and its
	// whole subtree).
	Ignore []string

	// Ctx cancels an in-progress scan (may be nil).
	Ctx context.Context

	// Log receives status lines for very long progress prints (may be nil).
	Log func(format string, args ...interface{})

	// Progress is invoked after each entry is hashed (may be nil).
	Progress func(Progress)
}

// compileIgnores normalizes user-supplied ignore patterns. Patterns are
// trimmed, converted to forward slashes and stripped of leading/trailing
// slashes.
func compileIgnores(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = filepath.ToSlash(strings.TrimSpace(p))
		p = strings.Trim(p, "/")
		p = strings.TrimPrefix(p, "./")
		if p == "" || p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}

// matchesIgnore reports whether the entry named name at slash-separated
// relative path rel matches one of the normalized patterns. A pattern matches
// the entry either by name (anywhere in the tree) or as a prefix of the
// relative path (covering the whole subtree).
func matchesIgnore(rel, name string, patterns []string) bool {
	for _, p := range patterns {
		if p == name || p == rel || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// ScanError reports a failure to run a scan. Walk reports whether the error
// came from traversing the directory tree (which maps to exit code 2).
type ScanError struct {
	Walk bool
	err  error
}

func (e *ScanError) Error() string { return e.err.Error() }
func (e *ScanError) Unwrap() error { return e.err }

// Run walks SourceDir, hashes every regular file, writes the report files
// into OutputDir and returns a structured Result.
func Run(o Options) (*Result, error) {
	start := time.Now()
	ctx := o.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	sourceDir := o.SourceDir
	outputDir := o.OutputDir

	logf := func(format string, args ...interface{}) {
		if o.Log != nil {
			o.Log(format, args...)
		}
	}
	reportProgress := func(p Progress) {
		if o.Progress != nil {
			o.Progress(p)
		}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating output directory: %w", err)}
	}

	// Errors: collect for the structured Result and stream to errors.txt.
	errorsFile, err := os.Create(filepath.Join(outputDir, "errors.txt"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating errors.txt: %w", err)}
	}
	defer errorsFile.Close()

	emptyDirsFile, err := os.Create(filepath.Join(outputDir, "empty-dirs.txt"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating empty-dirs.txt: %w", err)}
	}
	defer emptyDirsFile.Close()

	emptyFilesFile, err := os.Create(filepath.Join(outputDir, "empty-files.txt"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating empty-files.txt: %w", err)}
	}
	defer emptyFilesFile.Close()

	duplicatesFile, err := os.Create(filepath.Join(outputDir, "duplicates.txt"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating duplicates.txt: %w", err)}
	}
	defer duplicatesFile.Close()

	filesFile, err := os.Create(filepath.Join(outputDir, "files.txt"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating files.txt: %w", err)}
	}
	defer filesFile.Close()

	rmDuplicatesFile, err := os.Create(filepath.Join(outputDir, "rm-duplicates.sh"))
	if err != nil {
		return nil, &ScanError{err: fmt.Errorf("error creating rm-duplicates.sh: %w", err)}
	}
	defer rmDuplicatesFile.Close()

	sourceAbs, _ := filepath.Abs(sourceDir)
	fmt.Fprintf(rmDuplicatesFile, "#!/bin/bash\n\n# This script deletes duplicate files listed in duplicates.txt\n# Review the file before running this script!\n\n# Paths are relative to the scanned source directory:\ncd %s || exit 1\n\n", shellQuote(sourceAbs))

	var errs []string
	errorf := func(format string, args ...interface{}) {
		line := fmt.Sprintf(format, args...)
		errs = append(errs, line)
		fmt.Fprintln(errorsFile, line)
	}

	sourcePrefix := filepath.Clean(sourceDir) + string(filepath.Separator)
	outputAbs, _ := filepath.Abs(outputDir) // skip our own output dir if it lives inside the source tree

	logf("Starting scan of %s...", sourceDir)

	ignorePatterns := compileIgnores(o.Ignore)

	var files []FileInfo
	var bytesHashed int64
	walkErr := filepath.Walk(sourceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			errorf("skipping: %v", err)
			if info != nil && info.IsDir() {
				return filepath.SkipDir // skip the entire directory if it can't be accessed
			}
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if info.Mode()&os.ModeSymlink != 0 {
			errorf("skipping symlink: %s", filePath)
			return nil
		}

		rel := filepath.ToSlash(strings.TrimPrefix(filePath, sourcePrefix))
		name := info.Name()

		if info.IsDir() {
			if matchesIgnore(rel, name, ignorePatterns) {
				errorf("skipping ignored directory: %s", filePath)
				return filepath.SkipDir
			}
			absPath, _ := filepath.Abs(filePath)
			if systemDirs[absPath] {
				errorf("skipping system directory: %s", filePath)
				return filepath.SkipDir
			}
			if ignoredDirs[info.Name()] {
				errorf("skipping ignored directory: %s", filePath)
				return filepath.SkipDir
			}
			if absPath == outputAbs {
				errorf("skipping output directory: %s", filePath)
				return filepath.SkipDir // skip our own output directory
			}
			if filePath == sourceDir {
				return nil // skip the root directory itself
			}
			files = append(files, FileInfo{
				Hash: dirHash,
				Size: 0,
				Path: filepath.ToSlash(strings.TrimPrefix(filePath, sourcePrefix)),
			})
			return nil
		}

		if !info.Mode().IsRegular() {
			errorf("skipping non-regular file: %s", filePath)
			return nil
		}

		if matchesIgnore(rel, name, ignorePatterns) {
			errorf("skipping ignored file: %s", filePath)
			return nil
		}

		f, err := os.Open(filePath)
		if err != nil {
			errorf("skipping file: %v", err)
			return nil
		}

		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		f.Close() // close immediately: the walk may open many files
		if copyErr != nil {
			errorf("read error: %v", copyErr)
			return nil
		}

		files = append(files, FileInfo{
			Hash: fmt.Sprintf("%x", h.Sum(nil)),
			Size: info.Size(),
			Path: filepath.ToSlash(strings.TrimPrefix(filePath, sourcePrefix)),
		})
		bytesHashed += info.Size()

		reportProgress(Progress{
			Hashed:      len(files),
			BytesHashed: bytesHashed,
			CurrentPath: filePath,
			ErrorCount:  len(errs),
			LastError:   lastError(errs),
			Elapsed:     time.Since(start),
		})
		if len(files)%100 == 0 {
			logf("%d files hashed...", len(files))
		}
		return nil
	})
	if walkErr != nil {
		errorf("error: %v", walkErr)
		return nil, &ScanError{Walk: true, err: walkErr}
	}

	logf("Hashed %d files in %s", len(files), formatDuration(time.Since(start)))

	// Group files by hash, ignoring directory entries.
	hashGroups := make(map[string][]FileInfo)
	for _, f := range files {
		if f.Size == 0 && f.Hash == dirHash {
			continue // directories
		}
		hashGroups[f.Hash] = append(hashGroups[f.Hash], f)
	}

	// Collect duplicate groups (2+ files with the same hash).
	var dups []DupGroup
	for hash, group := range hashGroups {
		if len(group) < 2 {
			continue // not a duplicate
		}
		size := group[0].Size
		count := len(group)
		waste := size * int64(count-1)
		var paths []string
		for _, f := range group {
			if f.Size != size {
				errorf("hash collision: %s has different sizes (%d vs %d)", hash, size, f.Size)
			}
			paths = append(paths, f.Path)
		}
		sort.Strings(paths)
		dups = append(dups, DupGroup{
			Hash:  hash,
			Size:  size,
			Count: count,
			Waste: waste,
			Paths: paths,
		})
	}

	// Sort by wasted space, descending.
	sort.Slice(dups, func(i, j int) bool {
		return dups[i].Waste > dups[j].Waste
	})

	var totalWaste int64
	for _, d := range dups {
		totalWaste += d.Waste
	}

	// Write the duplicate reports and the removal script.
	if len(dups) > 0 {
		fmt.Fprintf(duplicatesFile, "=== %d Duplicate Files (%.2f MiB wasted) ===\n\n", len(dups), float64(totalWaste)/(1024*1024))
		for _, d := range dups {
			sizeMB := float64(d.Size) / (1024 * 1024)
			fmt.Fprintf(duplicatesFile, "%.2f MiB = %d x %.2f MiB\t%s\n", sizeMB*float64(d.Count), d.Count, sizeMB, d.Hash)
			first := true
			for _, p := range d.Paths {
				fmt.Fprintf(duplicatesFile, "\t%s\n", p)
				if first {
					fmt.Fprintf(rmDuplicatesFile, "# Keep: %s %s\n", shellQuote(p), d.Hash)
					first = false
				} else {
					fmt.Fprintf(rmDuplicatesFile, "rm %s\n", shellQuote(p))
				}
			}
			fmt.Fprintln(duplicatesFile)
			fmt.Fprintln(rmDuplicatesFile)
		}
	}
	duplicatesFile.Close()
	rmDuplicatesFile.Close()

	// Collect stats.
	var numFiles, numDirs, numEmpty int
	for _, f := range files {
		if f.Size == 0 && f.Hash == dirHash {
			numDirs++
		} else {
			numFiles++
			if f.Size == 0 {
				numEmpty++
			}
		}
	}

	// Sort the full file list by path.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	// Find empty directories (directories with no entry beneath them).
	dirSet := make(map[string]bool)
	for _, f := range files {
		if f.Hash == dirHash {
			dirSet[f.Path] = true
		}
	}
	hasChildren := make(map[string]bool)
	for _, f := range files {
		parent := path.Dir(f.Path)
		for parent != "." && parent != "" {
			hasChildren[parent] = true
			parent = path.Dir(parent)
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
	emptyDirsFile.Close()

	// Write the full file list and the empty file list.
	fmt.Fprintf(filesFile, "=== %d files, %d directories, %d empty files, %d empty directories ===\n", numFiles, numDirs, numEmpty, len(emptyDirs))
	fmt.Fprintf(emptyFilesFile, "=== %d empty files === %s\n\n", numEmpty, emptyHash)
	var emptyFiles []string
	for _, f := range files {
		fmt.Fprintf(filesFile, "%s\t%d\t%s\n", f.Hash, f.Size, f.Path)
		if f.Size == 0 && f.Hash != dirHash {
			emptyFiles = append(emptyFiles, f.Path)
			fmt.Fprintln(emptyFilesFile, f.Path)
		}
	}
	filesFile.Close()
	emptyFilesFile.Close()

	return &Result{
		Files:      files,
		DupGroups:  dups,
		EmptyFiles: emptyFiles,
		EmptyDirs:  emptyDirs,
		Errors:     errs,
		Stats: Stats{
			TotalFiles:      numFiles,
			TotalDirs:       numDirs,
			EmptyFiles:      numEmpty,
			EmptyDirs:       len(emptyDirs),
			DuplicateGroups: len(dups),
			TotalWaste:      totalWaste,
			Elapsed:         time.Since(start),
		},
	}, nil
}

// shellQuote wraps s in single quotes for safe use in a POSIX shell,
// escaping any embedded single quotes. Handles spaces, $, `, \, etc.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// lastError returns the most recent error line, if any.
func lastError(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	return errs[len(errs)-1]
}

// formatDuration renders elapsed time as h/m/s.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	mins := int(d.Minutes()) - hours*60
	secs := d.Seconds() - float64(hours*3600+mins*60)
	return fmt.Sprintf("%dh%dm%.3fs", hours, mins, secs)
}
