package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunFindsDuplicatesEmptyFilesAndDirs(t *testing.T) {
	src := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")

	// Two identical files, one unique file, one empty file, one empty dir.
	writeFile(t, filepath.Join(src, "a.txt"), "hello")
	writeFile(t, filepath.Join(src, "sub", "a-copy.txt"), "hello")
	writeFile(t, filepath.Join(src, "b.txt"), "unique content")
	writeFile(t, filepath.Join(src, "empty.txt"), "")
	if err := os.MkdirAll(filepath.Join(src, "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Run(Options{SourceDir: src, OutputDir: out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Stats.DuplicateGroups != 1 {
		t.Errorf("expected 1 duplicate group, got %d", res.Stats.DuplicateGroups)
	}
	if len(res.DupGroups) != 1 {
		t.Fatalf("expected 1 dup group, got %d", len(res.DupGroups))
	}
	for _, p := range res.DupGroups[0].Paths {
		if p != "a.txt" && p != "sub/a-copy.txt" {
			t.Errorf("unexpected path in duplicate group: %s", p)
		}
	}

	if res.Stats.EmptyFiles != 1 || len(res.EmptyFiles) != 1 {
		t.Errorf("expected 1 empty file, got stats=%d list=%d", res.Stats.EmptyFiles, len(res.EmptyFiles))
	}

	if res.Stats.EmptyDirs != 1 || len(res.EmptyDirs) != 1 || res.EmptyDirs[0] != "emptydir" {
		t.Errorf("expected empty dir 'emptydir', got %v", res.EmptyDirs)
	}

	// Reports must exist and contain expected headers.
	for _, name := range []string{
		"files.txt", "duplicates.txt", "empty-files.txt",
		"empty-dirs.txt", "errors.txt", "rm-duplicates.sh",
	} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("report %s not written: %v", name, err)
		}
	}

	dups, _ := os.ReadFile(filepath.Join(out, "duplicates.txt"))
	if !strings.Contains(string(dups), "1 Duplicate Files") {
		t.Errorf("duplicates.txt header missing: %q", dups)
	}

	rm, _ := os.ReadFile(filepath.Join(out, "rm-duplicates.sh"))
	if !strings.Contains(string(rm), "rm 'sub/a-copy.txt'") {
		t.Errorf("rm-duplicates.sh missing deletion for copy: %q", rm)
	}
}

func TestRunIgnoresGitAndSkipsSymlinks(t *testing.T) {
	src := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")

	writeFile(t, filepath.Join(src, ".git", "config"), "ignored")
	writeFile(t, filepath.Join(src, "real.txt"), "content")

	if err := os.Symlink("real.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	res, err := Run(Options{SourceDir: src, OutputDir: out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range res.Files {
		if strings.HasPrefix(f.Path, ".git") {
			t.Errorf(".git dir not skipped: %s", f.Path)
		}
		if f.Path == "link.txt" {
			t.Errorf("symlink should be skipped")
		}
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected errors for ignored dir/symlink, got none")
	}
}

func TestRunUsesUserIgnorePatterns(t *testing.T) {
	src := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")

	// node_modules and .git should be matched by name anywhere in the tree,
	// secret.env by filename, and src/vendor by relative path (with its
	// subtree).
	writeFile(t, filepath.Join(src, "node_modules", "pkg", "index.js"), "mod")
	writeFile(t, filepath.Join(src, "src", "node_modules", "deep.js"), "deep")
	writeFile(t, filepath.Join(src, ".git", "config"), "git")
	writeFile(t, filepath.Join(src, "secret.env"), "secret")
	writeFile(t, filepath.Join(src, "src", "vendor", "lib.so"), "binary")
	writeFile(t, filepath.Join(src, "src", "vendor", "deep", "extra.bin"), "more")
	writeFile(t, filepath.Join(src, "keep.txt"), "keep")

	res, err := Run(Options{
		SourceDir: src,
		OutputDir: out,
		Ignore:    []string{"node_modules", ".git", "secret.env", "src/vendor"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	found := false
	for _, f := range res.Files {
		switch {
		case strings.Contains(f.Path, "node_modules"):
			t.Errorf("node_modules not ignored: %s", f.Path)
		case strings.HasPrefix(f.Path, ".git"):
			t.Errorf(".git not ignored: %s", f.Path)
		case f.Path == "secret.env":
			t.Errorf("secret.env not ignored: %s", f.Path)
		case strings.HasPrefix(f.Path, "src/vendor"):
			t.Errorf("src/vendor subtree not ignored: %s", f.Path)
		case f.Path == "keep.txt":
			found = true
		}
	}
	if !found {
		t.Errorf("keep.txt should have been scanned")
	}
}

func TestCompileIgnoresNormalizesPatterns(t *testing.T) {
	got := compileIgnores([]string{" node_modules/ ", "/.git", "./src/", "", "."})
	want := []string{"node_modules", ".git", "src"}
	if len(got) != len(want) {
		t.Fatalf("expected %d patterns, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pattern %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestNoReportsHeadersWhenNoDuplicates(t *testing.T) {
	src := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	writeFile(t, filepath.Join(src, "one.txt"), "only one")

	res, err := Run(Options{SourceDir: src, OutputDir: out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.DupGroups) != 0 {
		t.Errorf("expected no duplicate groups, got %d", len(res.DupGroups))
	}
	if res.Stats.DuplicateGroups != 0 {
		t.Errorf("expected 0 duplicate groups in stats, got %d", res.Stats.DuplicateGroups)
	}
}

func TestProgressCallbackFires(t *testing.T) {
	src := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	writeFile(t, filepath.Join(src, "one.txt"), "one")

	var last Progress
	_, err := Run(Options{
		SourceDir: src,
		OutputDir: out,
		Progress:  func(p Progress) { last = p },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if last.Hashed == 0 || last.BytesHashed == 0 {
		t.Errorf("expected progress to fire with hashes, got %+v", last)
	}
}
