package tui

import (
	"fmt"
	"strings"

	"github.com/fffaraz/dupdig/internal/scan"
)

// ---------------------------------------------------------------------------
// Whole-screen rendering
// ---------------------------------------------------------------------------

func (m *model) resultsView() string {
	res := m.result
	if res == nil {
		return ""
	}

	m.ensureFiltered()

	w, h := m.width, m.height
	footer := m.footerLine(w)
	footerLines := strings.Count(footer, "\n") + 1
	listH := h - 3 - footerLines
	if listH < 0 {
		listH = 0
	}
	content, cursorLine := m.tabLines(w)
	top := m.windowTop(cursorLine, listH)
	m.top = top

	lines := make([]string, 0, 3+listH+footerLines)

	lines = append(lines, titleStyle.Render("dupdig")+"  "+
		headerStyle.Render(truncate(m.sourceDir, maxInt(1, w-80)))+" "+
		headerStyle.Render("→")+" "+
		headerStyle.Render(truncate(m.outputDir, w)))

	lines = append(lines, tabBar(m.tab, m.counts()))
	lines = append(lines, emptyDivider(w))

	for i := 0; i < listH; i++ {
		if i+top < len(content) {
			lines = append(lines, content[i+top])
		} else {
			lines = append(lines, "")
		}
	}

	lines = append(lines, strings.Split(footer, "\n")...)

	return fillScreen(lines, w, h)
}

// windowTop keeps the cursor row inside the visible content window.
func (m *model) windowTop(cursorLine, listH int) int {
	top := m.top
	if cursorLine < top {
		top = cursorLine
	}
	if cursorLine >= top+listH {
		top = cursorLine - listH + 1
	}
	return maxInt(0, top)
}

// fillScreen pads/truncates lines to exactly width x height.
func fillScreen(lines []string, w, h int) string {
	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		var s string
		if i < len(lines) {
			s = lines[i]
		}
		out = append(out, padRight(truncate(s, w), w))
	}
	return strings.Join(out, "\n")
}

// tabLines returns the rows for the active tab and the line index of the
// selected row.
func (m *model) tabLines(w int) ([]string, int) {
	res := m.result
	if res == nil {
		return nil, 0
	}
	switch m.tab {
	case tabSummary:
		return m.summaryLines(w), 0
	case tabDuplicates:
		return m.dupLines(w)
	case tabFiles:
		return m.fileLines(w)
	case tabEmptyFiles:
		return m.listLines(w, res.EmptyFiles, m.idxEF)
	case tabEmptyDirs:
		return m.listLines(w, res.EmptyDirs, m.idxED)
	case tabErrors:
		return m.listLines(w, res.Errors, m.idxEr)
	}
	return nil, 0
}

// counts returns per-tab totals for the tab strip.
func (m *model) counts() map[TabID]int {
	out := make(map[TabID]int)
	if m.result == nil {
		return out
	}
	out[tabSummary] = m.result.Stats.TotalFiles
	out[tabDuplicates] = len(m.result.DupGroups)
	out[tabFiles] = len(m.result.Files)
	out[tabEmptyFiles] = len(m.result.EmptyFiles)
	out[tabEmptyDirs] = len(m.result.EmptyDirs)
	out[tabErrors] = len(m.result.Errors)
	return out
}

func (m *model) footerLine(w int) string {
	keys := "[1-6] tabs   [/] search   [r] rescan   [q] quit"
	if m.tab == tabDuplicates {
		keys = "[1-6] tabs   [/] search   [enter] expand   [r] rescan   [q] quit"
	}

	var b strings.Builder
	b.WriteString(keyStyle.Render(keys))
	if m.tab != tabSummary {
		right := fmt.Sprintf("%d / %d", m.cursor+1, maxInt(1, m.rowCount()))
		pad := maxInt(0, w-(len(keys)+len(right)+4))
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(accStyle.Render(right))
		}
	}

	if m.searchFocus || m.filter != "" {
		filter := m.filter
		if m.searchFocus {
			filter += "▌"
		}
		b.WriteByte('\n')
		b.WriteString(lblStyle.Render("filter: "))
		b.WriteString(accStyle.Render(filter))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func (m *model) summaryLines(w int) []string {
	res := m.result
	rows := [][2]string{
		{"Files", comma(res.Stats.TotalFiles)},
		{"Directories", comma(res.Stats.TotalDirs)},
		{"Empty files", comma(res.Stats.EmptyFiles)},
		{"Empty directories", comma(res.Stats.EmptyDirs)},
		{"Errors", comma(len(res.Errors))},
	}
	var s []string
	s = append(s, "")
	s = append(s, valStyle.Render("Duplicate groups: ")+accStyle.Render(comma(res.Stats.DuplicateGroups))+
		accStyle.Render("    reclaimable: ")+humanWaste(res.Stats.TotalWaste))
	s = append(s, "")
	for _, r := range rows {
		s = append(s, "  "+lblStyle.Render(padRight(r[0], 22))+valStyle.Render(r[1]))
	}
	s = append(s, "")
	s = append(s, lblStyle.Render("Scan time: ")+valStyle.Render(fmtDur(res.Stats.Elapsed)))
	s = append(s, "")
	s = append(s, headerStyle.Render("Reports written to "+truncate(m.outputDir, w-24)))
	return s
}

// ---------------------------------------------------------------------------
// Duplicates
// ---------------------------------------------------------------------------

func (m *model) dupLines(w int) ([]string, int) {
	res := m.result
	idx := m.idxDup
	if idx == nil {
		idx = identity(len(res.DupGroups))
	}
	if len(idx) == 0 {
		return []string{emptyLine("No duplicate groups match this filter.")}, 0
	}

	var lines []string
	lines = append(lines, headerStyle.Render(strings.Join([]string{
		padRight("Waste", 11),
		padRight("Size", 11),
		padRight("Copies", 8),
		padRight("Hash", 22),
	}, " ")))

	cursorLine := 0
	for i, gi := range idx {
		g := res.DupGroups[gi]
		sel := i == m.cursor
		if sel {
			cursorLine = len(lines)
		}
		lines = append(lines, m.dupGroupLine(g, sel, w))
		if m.expanded[gi] {
			for _, p := range g.Paths {
				lines = append(lines, dupPathLine(p, sel, w))
			}
		}
	}
	return lines, cursorLine
}

func (m *model) dupGroupLine(g scan.DupGroup, sel bool, w int) string {
	row := strings.Join([]string{
		padRight(humanBytes(g.Waste), 11),
		padRight(humanBytes(g.Size), 11),
		padRight(fmt.Sprintf("%d ×", g.Count), 8),
		truncate(g.Hash, 22),
	}, " ")
	row = truncate(row, w-2)
	if sel {
		return "▶ " + selStyle.Render(row)
	}
	return "  " + row
}

func dupPathLine(p string, sel bool, w int) string {
	row := "    └─ " + truncate(p, w-9)
	if sel {
		return row
	}
	return dimSelStyle.Render(row)
}

// ---------------------------------------------------------------------------
// Files and plain lists
// ---------------------------------------------------------------------------

func (m *model) fileLines(w int) ([]string, int) {
	res := m.result
	idx := m.idxF
	if idx == nil {
		idx = identity(len(res.Files))
	}
	if len(idx) == 0 {
		return []string{emptyLine("No files match this filter.")}, 0
	}

	var lines []string
	lines = append(lines, headerStyle.Render(strings.Join([]string{
		padRight("Path", 34),
		padRight("Size", 13),
		"Hash",
	}, " ")))

	cursorLine := 0
	for i, fi := range idx {
		f := res.Files[fi]
		sel := i == m.cursor
		if sel {
			cursorLine = len(lines)
		}
		row := strings.Join([]string{
			padRight(truncate(f.Path, 33), 34),
			padRight(humanBytes(f.Size), 13),
			truncate(f.Hash, 64),
		}, "")
		row = truncate(row, w-2)
		if sel {
			lines = append(lines, "▶ "+selStyle.Render(row))
		} else {
			lines = append(lines, "  "+row)
		}
	}
	return lines, cursorLine
}

func (m *model) listLines(w int, items []string, idx []int) ([]string, int) {
	if idx == nil {
		idx = identity(len(items))
	}
	if len(idx) == 0 {
		return []string{emptyLine("Nothing matches this filter.")}, 0
	}

	lines := make([]string, 0, len(idx))
	cursorLine := 0
	for i, ii := range idx {
		sel := i == m.cursor
		if sel {
			cursorLine = len(lines)
		}
		row := truncate(items[ii], w-2)
		if sel {
			lines = append(lines, "▶ "+truncate(row, w-5))
		} else {
			lines = append(lines, "  "+row)
		}
	}
	return lines, cursorLine
}

func emptyLine(msg string) string {
	return lblStyle.Render(msg)
}

// identity returns indices 0..n-1 (nil for n == 0).
func identity(n int) []int {
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = i
	}
	return out
}
