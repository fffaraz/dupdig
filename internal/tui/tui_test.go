package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fffaraz/dupdig/internal/scan"
)

const fakeHash = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func sampleResult() *scan.Result {
	return &scan.Result{
		DupGroups: []scan.DupGroup{
			{Hash: fakeHash, Size: 20, Count: 3, Waste: 60, Paths: []string{"c/a.txt", "c/b.txt", "c/c.txt"}},
			{Hash: strings.Repeat("f", 64), Size: 5, Count: 2, Waste: 5, Paths: []string{"small/x", "small/y"}},
		},
		Files: []scan.FileInfo{
			{Hash: fakeHash, Size: 20, Path: "c/a.txt"},
			{Hash: fakeHash, Size: 20, Path: "c/b.txt"},
			{Hash: fakeHash, Size: 20, Path: "c/c.txt"},
			{Hash: "aa", Size: 5, Path: "small/x.txt"},
			{Hash: "bb", Size: 0, Path: "empty.txt"},
		},
		EmptyFiles: []string{"empty.txt"},
		EmptyDirs:  []string{"emptydir"},
		Errors:     []string{"skipping ignored directory: .git"},
		Stats: scan.Stats{
			TotalFiles:      5,
			TotalDirs:       2,
			EmptyFiles:      1,
			EmptyDirs:       1,
			DuplicateGroups: 2,
			TotalWaste:      65,
		},
	}
}

func newTestModel(res *scan.Result) *model {
	m := newModel("/src", "/out", nil)
	m.width, m.height = 80, 24
	m.result = res
	m.scanning = false
	return m
}

func TestSummaryView(t *testing.T) {
	m := newTestModel(sampleResult())
	v := m.View()
	for _, want := range []string{"dupdig", "Summary", "Duplicate groups: 2", "65 B"} {
		if !strings.Contains(v, want) {
			t.Errorf("summary missing %q", want)
		}
	}
}

func TestDuplicatesView(t *testing.T) {
	m := newTestModel(sampleResult())
	m.tab = tabDuplicates
	v := m.View()
	for _, want := range []string{"Waste", "Copies", "3 "} {
		if !strings.Contains(v, want) {
			t.Errorf("duplicates missing %q", want)
		}
	}
}

func TestExpandPaths(t *testing.T) {
	m := newTestModel(sampleResult())
	m.tab = tabDuplicates
	m.expanded[0] = true
	v := m.View()
	for _, want := range []string{"c/a.txt", "c/b.txt", "c/c.txt"} {
		if !strings.Contains(v, want) {
			t.Errorf("expanded paths missing %q", want)
		}
	}
}

func TestFilter(t *testing.T) {
	m := newTestModel(sampleResult())
	m.tab = tabFiles
	m.searchFocus = true
	m.filter = "empty"
	v := m.View()
	if !strings.Contains(v, "empty.txt") {
		t.Errorf("filtered files view should contain empty.txt")
	}
}

func TestKeyNavigation(t *testing.T) {
	m := newTestModel(sampleResult())
	m.tab = tabDuplicates
	m.cursor = 0
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}})
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}
	if m.rowCount() != 2 {
		t.Errorf("expected 2 duplicate rows, got %d", m.rowCount())
	}
}

func TestScanProgressView(t *testing.T) {
	m := newTestModel(nil)
	m.scanning = true
	m.progress = scan.Progress{Hashed: 42, BytesHashed: 1337, CurrentPath: "/src/file.bin", ErrorCount: 7}
	v := m.View()
	for _, want := range []string{"42", "file.bin", "7"} {
		if !strings.Contains(v, want) {
			t.Errorf("scan view missing %q", want)
		}
	}
}
