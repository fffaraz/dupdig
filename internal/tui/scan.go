package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

func spinner(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}

func emptyDivider(w int) string {
	return lipgloss.NewStyle().Foreground(grayColor).Render(strings.Repeat("─", maxInt(0, w-2)))
}

// scanView renders the live progress screen while hashing.
func (m *model) scanView() string {
	w := m.width
	bodyW := maxInt(1, w-2)

	var b strings.Builder
	b.WriteString(titleStyle.Render("dupdig"))
	b.WriteByte('\n')
	b.WriteString(headerStyle.Render("scanning " + truncate(m.sourceDir, w-40)))
	b.WriteByte('\n')
	b.WriteString(emptyDivider(w))
	b.WriteByte('\n')

	p := m.progress
	b.WriteString(accStyle.Render(spinner(m.frame) + " hashing files…"))
	b.WriteByte('\n')
	if p.Hashed > 0 {
		b.WriteString(indeterminateBar(bodyW, m.frame))
	} else {
		b.WriteString(barEmpty().Width(bodyW).Render(strings.Repeat(" ", bodyW)))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	line := lblStyle.Render("entries  ") + valStyle.Render(padRight(comma(p.Hashed), 11)) +
		lblStyle.Render(" bytes  ") + valStyle.Render(padRight(humanBytes(p.BytesHashed), 12)) +
		lblStyle.Render(" elapsed  ") + valStyle.Render(padRight(fmtDur(p.Elapsed), 10)) +
		lblStyle.Render(" errors  ") + valStyle.Render(comma(p.ErrorCount))
	b.WriteString(line)
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(lblStyle.Render("current: "))
	b.WriteString(truncate(p.CurrentPath, bodyW-9))
	b.WriteByte('\n')

	if p.LastError != "" {
		b.WriteString(errorStyle.Render("  ! " + truncate(p.LastError, bodyW-3)))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(keyStyle.Render("[ctrl+c] cancel   [q] quit"))

	return lipgloss.NewStyle().Padding(1, 1).Render(strings.TrimSuffix(b.String(), "\n"))
}

// errorView renders a fatal scan failure.
func (m *model) errorView() string {
	w := m.width
	msg := "scan failed"
	if m.scanErr != nil {
		msg = m.scanErr.Error()
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("dupdig"))
	b.WriteByte('\n')
	b.WriteString(emptyDivider(w))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(errorStyle.Render("✗ " + truncate(msg, maxInt(1, w-6))))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(keyStyle.Render("press q to quit"))
	return lipgloss.NewStyle().Padding(1, 1).Render(strings.TrimSuffix(b.String(), "\n"))
}
