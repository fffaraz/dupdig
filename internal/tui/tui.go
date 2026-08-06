package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fffaraz/dupdig/internal/scan"
)

// TabID identifies one results tab.
type TabID int

const (
	tabSummary TabID = iota
	tabDuplicates
	tabFiles
	tabEmptyFiles
	tabEmptyDirs
	tabErrors
	tabCount
)

var tabLabels = [tabCount]string{
	"Summary",
	"Duplicates",
	"Files",
	"Empty files",
	"Empty dirs",
	"Errors",
}

func tabLabel(t TabID) string {
	if t < 0 || int(t) >= int(tabCount) {
		return "?"
	}
	return tabLabels[t]
}

// scanMsg is delivered when a background scan finishes.
type scanMsg struct {
	res *scan.Result
	err error
}

// tickMsg drives the scanning animation.
type tickMsg struct{}

// model is the root bubbletea model for the dupdig TUI.
type model struct {
	sourceDir string
	outputDir string
	ignore    []string

	progCh chan scan.Progress
	doneCh chan scanMsg

	ctx    context.Context
	cancel context.CancelFunc

	scanning bool
	progress scan.Progress
	result   *scan.Result
	scanErr  error
	frame    int

	width  int
	height int

	searchFocus bool
	filter      string

	tab      TabID
	cursor   int
	top      int
	expanded map[int]bool

	// caching of the filtered index lists (one per list tab)
	qIdx   string
	resIdx *scan.Result
	idxDup []int
	idxF   []int
	idxEF  []int
	idxED  []int
	idxEr  []int
}

// UI runs the interactive terminal interface until the user quits.
func UI(sourceDir, outputDir string, ignore []string) error {
	p := tea.NewProgram(newModel(sourceDir, outputDir, ignore), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(sourceDir, outputDir string, ignore []string) *model {
	return &model{
		sourceDir: sourceDir,
		outputDir: outputDir,
		ignore:    ignore,
		tab:       tabSummary,
		expanded:  make(map[int]bool),
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.startScan(), m.tickCmd())
}

// startScan launches a background scan and swaps in fresh channels.
func (m *model) startScan() tea.Cmd {
	m.cancelScan()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel = ctx, cancel
	m.progCh = make(chan scan.Progress, 1024)
	m.doneCh = make(chan scanMsg, 1)
	m.scanning = true
	m.result = nil
	m.scanErr = nil
	m.progress = scan.Progress{}
	m.frame = 0
	m.expanded = make(map[int]bool)
	m.invalidateCache()

	go func() {
		res, err := scan.Run(scan.Options{
			SourceDir: m.sourceDir,
			OutputDir: m.outputDir,
			Ignore:    m.ignore,
			Ctx:       ctx,
			Progress: func(p scan.Progress) {
				select {
				case m.progCh <- p:
				default: // drop progress events when the UI runs behind
				}
			},
		})
		m.doneCh <- scanMsg{res: res, err: err}
	}()

	return tea.Batch(m.progressCmd(), m.tickCmd())
}

func (m *model) cancelScan() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m *model) progressCmd() tea.Cmd {
	return func() tea.Msg {
		p, ok := <-m.progCh
		if !ok {
			return nil
		}
		return p
	}
}

func (m *model) tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	case scan.Progress:
		if m.scanning {
			m.progress = msg
			return m, m.progressCmd()
		}
	case scanMsg:
		m.scanning = false
		m.result = msg.res
		m.scanErr = msg.err
		m.cursor, m.top = 0, 0
		if m.expanded == nil {
			m.expanded = make(map[int]bool)
		}
	case tickMsg:
		if m.scanning {
			m.frame++
			return m, m.tickCmd()
		}
	}
	return m, nil
}

func (m *model) View() string {
	if m.width <= 0 {
		m.width, m.height = 80, 24
	}
	switch {
	case m.result == nil && m.scanning:
		return m.scanView()
	case m.result == nil && m.scanErr != nil:
		return m.errorView()
	default:
		return m.resultsView()
	}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.cancelScan()
		return m, tea.Quit
	}

	// While scanning, navigation is disabled.
	if m.scanning {
		if msg.Type == tea.KeyRunes && (msg.Runes[0] == 'q' || msg.Runes[0] == 'Q') {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.searchFocus {
		return m.chip(msg)
	}

	if msg.Type == tea.KeyRunes {
		switch msg.Runes[0] {
		case 'q', 'Q':
			return m, tea.Quit
		case '/':
			m.searchFocus = true
			m.filter = ""
			m.cursor, m.top = 0, 0
			return m, nil
		case 'r', 'R':
			return m, m.startScan()
		case 'g':
			m.cursor, m.top = 0, 0
			return m, nil
		case 'G':
			m.cursor = m.rowCount() - 1
			m.top = m.cursor
			return m, nil
		case '1', '2', '3', '4', '5', '6':
			m.tab = TabID(int(msg.Runes[0] - '1'))
			m.cursor, m.top = 0, 0
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyTab:
		m.tab = (m.tab + 1) % TabID(tabCount)
		m.cursor, m.top = 0, 0
	case tea.KeyUp:
		m.moveCursor(-1)
	case tea.KeyDown:
		m.moveCursor(1)
	case tea.KeyPgUp:
		m.moveCursor(-m.pageSize())
	case tea.KeyPgDown:
		m.moveCursor(m.pageSize())
	case tea.KeyHome:
		m.cursor, m.top = 0, 0
	case tea.KeyEnd:
		m.cursor = m.rowCount() - 1
		m.top = m.cursor
	case tea.KeyEnter:
		if m.tab == tabDuplicates {
			m.toggleExpanded()
		}
	}
	return m, nil
}

func (m *model) chip(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape, tea.KeyEnter:
		m.searchFocus = false
		if msg.Type == tea.KeyEscape {
			m.filter = ""
		}
		m.cursor, m.top = 0, 0
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.cursor, m.top = 0, 0
		}
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		m.cursor, m.top = 0, 0
	}
	return m, nil
}

// ensureFiltered recomputes the per-tab filtered index lists when the filter
// or the result changes.
func (m *model) ensureFiltered() {
	if m.qIdx == m.filter && m.resIdx == m.result {
		return
	}
	m.idxDup, m.idxF, m.idxEF, m.idxED, m.idxEr = nil, nil, nil, nil, nil
	if m.result == nil {
		m.qIdx, m.resIdx = m.filter, nil
		return
	}
	res := m.result
	q := strings.ToUpper(strings.TrimSpace(m.filter))
	m.qIdx, m.resIdx = m.filter, m.result
	if q == "" {
		return
	}
	build := func(n int, ok func(i int) bool) []int {
		out := make([]int, 0, 16)
		for i := 0; i < n; i++ {
			if ok(i) {
				out = append(out, i)
			}
		}
		return out
	}
	m.idxDup = build(len(res.DupGroups), func(i int) bool {
		g := res.DupGroups[i]
		if strings.Contains(strings.ToUpper(g.Hash), q) {
			return true
		}
		for _, p := range g.Paths {
			if strings.Contains(strings.ToUpper(p), q) {
				return true
			}
		}
		return false
	})
	m.idxF = build(len(res.Files), func(i int) bool {
		f := res.Files[i]
		return strings.Contains(strings.ToUpper(f.Path), q) || strings.Contains(strings.ToUpper(f.Hash), q)
	})
	m.idxEF = build(len(res.EmptyFiles), func(i int) bool {
		return strings.Contains(strings.ToUpper(res.EmptyFiles[i]), q)
	})
	m.idxED = build(len(res.EmptyDirs), func(i int) bool {
		return strings.Contains(strings.ToUpper(res.EmptyDirs[i]), q)
	})
	m.idxEr = build(len(res.Errors), func(i int) bool {
		return strings.Contains(strings.ToUpper(res.Errors[i]), q)
	})
}

func (m *model) invalidateCache() {
	m.qIdx = ""
	m.resIdx = nil
	m.idxDup, m.idxF, m.idxEF, m.idxED, m.idxEr = nil, nil, nil, nil, nil
}

func (m *model) rowCount() int {
	if m.result == nil {
		return 0
	}
	switch m.tab {
	case tabSummary:
		return 0
	case tabDuplicates:
		if m.idxDup == nil {
			return len(m.result.DupGroups)
		}
		return len(m.idxDup)
	case tabFiles:
		if m.idxF == nil {
			return len(m.result.Files)
		}
		return len(m.idxF)
	case tabEmptyFiles:
		if m.idxEF == nil {
			return len(m.result.EmptyFiles)
		}
		return len(m.idxEF)
	case tabEmptyDirs:
		if m.idxED == nil {
			return len(m.result.EmptyDirs)
		}
		return len(m.idxED)
	case tabErrors:
		if m.idxEr == nil {
			return len(m.result.Errors)
		}
		return len(m.idxEr)
	}
	return 0
}

func (m *model) moveCursor(delta int) {
	max := m.rowCount() - 1
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > max {
		m.cursor = max
	}
}

func (m *model) toggleExpanded() {
	if m.result == nil {
		return
	}
	indices := m.idxDup
	if indices == nil {
		indices = identity(len(m.result.DupGroups))
	}
	if m.cursor < 0 || m.cursor >= len(indices) {
		return
	}
	g := indices[m.cursor]
	if m.expanded[g] {
		delete(m.expanded, g)
	} else {
		m.expanded[g] = true
	}
}

func (m *model) pageSize() int {
	if h := m.listHeight(); h > 2 {
		return h - 2
	}
	return 1
}

// listHeight is how many content lines a results tab can use.
func (m *model) listHeight() int {
	return maxInt(1, m.height-4)
}

// matches reports whether s (any case) contains the already-uppercased query.
func matches(s, query string) bool {
	return strings.Contains(strings.ToUpper(s), query)
}
