package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accentColor = lipgloss.Color("#B18CFF")
	greenColor  = lipgloss.Color("#7CF5A5")
	redColor    = lipgloss.Color("#FF7A7A")
	amberColor  = lipgloss.Color("#FFD479")
	grayColor   = lipgloss.Color("#8A8F98")
	darkColor   = lipgloss.Color("#26252B")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Faint(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Faint(true)

	valStyle = lipgloss.NewStyle().
			Foreground(whiteColor()).
			Bold(true)

	lblStyle = lipgloss.NewStyle().
			Foreground(grayColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(amberColor).
			Bold(true)

	selStyle = lipgloss.NewStyle().
			Background(grayColor).
			Foreground(whiteColor()).
			Bold(true)

	accStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	dimSelStyle = lipgloss.NewStyle().
			Background(darkColor).
			Foreground(whiteColor())

	hashStyle = lipgloss.NewStyle().
			Foreground(grayColor)

	dupHashStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	tabActiveStyle = lipgloss.NewStyle().
			Background(accentColor).
			Foreground(blackColor()).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(grayColor).
				Padding(0, 1)

	frameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(grayColor).
			Padding(0, 1)
)

func whiteColor() lipgloss.Color { return lipgloss.Color("#F2F2F2") }
func blackColor() lipgloss.Color { return lipgloss.Color("#100F14") }
func barFill() lipgloss.Style {
	return lipgloss.NewStyle().Background(greenColor).Foreground(greenColor)
}
func barEmpty() lipgloss.Style { return lipgloss.NewStyle().Background(lipgloss.Color("#3A3A41")) }
func barBlink() lipgloss.Style { return lipgloss.NewStyle().Background(lipgloss.Color("#8C77E8")) }

// progressBar renders a horizontal bar of the given width.
func progressBar(width int, filled float64) string {
	if width <= 0 {
		return ""
	}
	if filled < 0 {
		filled = 0
	}
	if filled > 1 {
		filled = 1
	}
	n := int(filled * float64(width))
	if n > width {
		n = width
	}
	e := barEmpty().Width(width - n).Render(strings.Repeat(" ", width-n))
	f := barFill().Width(n).Render(strings.Repeat(" ", n))
	return f + e
}

// indeterminateBar renders an animated scanning bar.
func indeterminateBar(width int, frame int) string {
	if width <= 0 {
		return ""
	}
	slot := 4
	if slot > width {
		slot = width
	}
	pos := frame % (width - slot + 1)
	empty := barEmpty().Width(width).Render(strings.Repeat(" ", width))
	fill := barBlink().Width(slot).Render(strings.Repeat(" ", slot))
	if pos == 0 {
		return fill + empty[slot:]
	}
	return empty[:pos] + fill + empty[pos+slot:]
}

// tabBar renders the tab strip.
func tabBar(active TabID, counts map[TabID]int) string {
	var b strings.Builder
	n := int(tabCount)
	for i := 0; i < n; i++ {
		tab := TabID(i)
		label := tabLabel(tab)
		if v, ok := counts[tab]; ok {
			label += " (" + comma(v) + ")"
		}
		if tab == active {
			b.WriteString(tabActiveStyle.Render(label))
		} else {
			b.WriteString(tabInactiveStyle.Render(label))
		}
		if i < n-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}
