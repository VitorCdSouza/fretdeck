package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The palette is the one an electric guitar is actually made of: brass for the
// frets and the accent, a warm off white for the text instead of a grey, sage
// for a note that landed and rust for one that did not.
//
// Every colour is adaptive. A fixed set would be unreadable on half the
// terminals out there, and this app is meant to sit open on a second screen.
var (
	colorBrass  = lipgloss.AdaptiveColor{Light: "#8A5A0B", Dark: "#D9A441"}
	colorInk    = lipgloss.AdaptiveColor{Light: "#241F1A", Dark: "#EDE6D8"}
	colorSubtle = lipgloss.AdaptiveColor{Light: "#6B6157", Dark: "#A2968A"}
	colorFaint  = lipgloss.AdaptiveColor{Light: "#A79B8E", Dark: "#5C5348"}
	colorSage   = lipgloss.AdaptiveColor{Light: "#3F7A3A", Dark: "#8FBF6F"}
	colorRust   = lipgloss.AdaptiveColor{Light: "#A33B22", Dark: "#D9603F"}
	colorCopper = lipgloss.AdaptiveColor{Light: "#9A5A24", Dark: "#CF8B4C"}
)

var (
	styleBrand   = lipgloss.NewStyle().Bold(true).Foreground(colorBrass)
	styleTab     = lipgloss.NewStyle().Foreground(colorSubtle)
	styleTabOn   = lipgloss.NewStyle().Bold(true).Foreground(colorInk)
	styleRule    = lipgloss.NewStyle().Foreground(colorFaint)
	styleInk     = lipgloss.NewStyle().Foreground(colorInk)
	styleSubtle  = lipgloss.NewStyle().Foreground(colorSubtle)
	styleFaint   = lipgloss.NewStyle().Foreground(colorFaint)
	styleAccent  = lipgloss.NewStyle().Foreground(colorBrass)
	styleHeading = lipgloss.NewStyle().Bold(true).Foreground(colorInk)
	styleOk      = lipgloss.NewStyle().Foreground(colorSage)
	styleBad     = lipgloss.NewStyle().Foreground(colorRust)
	styleWarn    = lipgloss.NewStyle().Foreground(colorCopper)
	styleHelp    = lipgloss.NewStyle().Foreground(colorFaint)

	// the tab behind the cursor is dimmed and what is coming is not, so the
	// eye is pulled forward instead of back
	styleTabPast = lipgloss.NewStyle().Foreground(colorFaint)
	styleTabNext = lipgloss.NewStyle().Foreground(colorSubtle)
	styleTabHere = lipgloss.NewStyle().Bold(true).Foreground(colorBrass)
	styleString  = lipgloss.NewStyle().Foreground(colorSubtle)
)

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}

func rule(width int) string {
	if width < 1 {
		width = 1
	}
	return styleRule.Render(strings.Repeat("─", width))
}

// pad spreads left and right to the full width, which is how every header on
// every screen puts its status on the right edge.
func pad(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// bar is the progress line under the tab. The filled part is drawn with the
// upper eighth block so it reads as a line and not as a wall of colour.
func bar(fraction float64, width int, style lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	filled := int(fraction * float64(width))
	return style.Render(strings.Repeat("▔", filled)) +
		styleFaint.Render(strings.Repeat("▁", width-filled))
}

// marker is the caret drawn over the string being played, so the eye finds the
// line without reading the fret numbers first.
const marker = "▾"
