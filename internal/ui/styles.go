package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func textinputBlink() tea.Cmd { return textinput.Blink }

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

	// the repeat mode paints the app in this instead of the brass, since a
	// mode that changes what the keys do has to be seen and not remembered
	colorAzure = lipgloss.AdaptiveColor{Light: "#1F5FA8", Dark: "#7FB2E5"}

	// the band the row under the cursor sits on
	colorRow = lipgloss.AdaptiveColor{Light: "#E7DBC3", Dark: "#332C23"}
)

// rowPaint is that band, and every piece of a row has to carry it
type rowPaint bool

// of hands back the style with the band behind it, since a background laid
// over a finished line ends at the first reset the styles inside it wrote
func (on rowPaint) of(style lipgloss.Style) lipgloss.Style {
	if !on {
		return style
	}
	return style.Background(colorRow)
}

// fill pads a line out to the width of the column it is in, with the band
// behind it, so the row under the cursor is painted across the whole column
func (on rowPaint) fill(line string, width int) string {
	gap := width - lipgloss.Width(line)
	if gap < 1 {
		return truncate(line, width)
	}
	return line + on.of(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap))
}

// pad is the pad of a row, with the gap between its halves painted too
func (on rowPaint) pad(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + on.of(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap)) + right
}

// the bar under a button, and the thicker one under the screen that is open.
// The button is only its name, its padding and this.
var (
	tabUnderline   = lipgloss.Border{Bottom: "─"}
	tabUnderlineOn = lipgloss.Border{Bottom: "━"}
)

// accent is the colour the app is recognised by, and repaint is what swaps it.
// The styles that carry it are built there and not here, so there is still one
// place the colours live.
var accent lipgloss.TerminalColor = colorBrass

func repaint(repeating bool) {
	accent = colorBrass
	if repeating {
		accent = colorAzure
	}

	styleBrand = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleAccent = lipgloss.NewStyle().Foreground(accent)
	styleTabHere = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleTabBoxOn = lipgloss.NewStyle().Border(tabUnderlineOn, false, false, true, false).
		BorderForeground(accent).Padding(0, tabPad)
}

func init() { repaint(false) }

var (
	// the four repaint owns, since the accent is what a mode changes
	styleBrand    lipgloss.Style
	styleAccent   lipgloss.Style
	styleTabHere  lipgloss.Style
	styleTabBoxOn lipgloss.Style

	styleTab   = lipgloss.NewStyle().Foreground(colorSubtle)
	styleTabOn = lipgloss.NewStyle().Bold(true).Foreground(colorInk)

	// a screen is a button and the border is the bar under it, which is a box
	// with the sides taken off: two rows instead of three, and nothing between
	// the name and the edge of the button but the padding
	styleTabBox = lipgloss.NewStyle().Border(tabUnderline, false, false, true, false).
			BorderForeground(colorFaint).Padding(0, tabPad)

	// the one button of the app, on the spotify screen with nobody logged in.
	// the border is faint and the label carries the accent, so the mode still
	// paints it without a fifth style to rebuild
	styleButton = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(colorFaint).Padding(0, 2)

	styleRule    = lipgloss.NewStyle().Foreground(colorFaint)
	styleInk     = lipgloss.NewStyle().Foreground(colorInk)
	styleSubtle  = lipgloss.NewStyle().Foreground(colorSubtle)
	styleFaint   = lipgloss.NewStyle().Foreground(colorFaint)
	styleHeading = lipgloss.NewStyle().Bold(true).Foreground(colorInk)
	styleOk      = lipgloss.NewStyle().Foreground(colorSage)
	styleBad     = lipgloss.NewStyle().Foreground(colorRust)
	styleWarn    = lipgloss.NewStyle().Foreground(colorCopper)
	styleHelp    = lipgloss.NewStyle().Foreground(colorFaint)

	// the tab behind the cursor is dimmed and what is coming is not, so the
	// eye is pulled forward instead of back
	styleTabPast = lipgloss.NewStyle().Foreground(colorFaint)
	styleTabNext = lipgloss.NewStyle().Foreground(colorSubtle)
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

// plural keeps the count and its noun agreeing, since a library of one is the
// state the app opens in the first time it is used.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// columnHead is the small caps title a section opens with, and what it has to
// say about itself against the right edge of the column it is drawn in.
func columnHead(title, right string, width int) string {
	left := "  " + styleAccent.Render(title)

	// what the head says about itself goes when the column is too narrow for
	// both, since the title is the half that says what the list is
	if right == "" || lipgloss.Width(left)+len(right)+4 > width {
		return left
	}

	return pad(left, styleFaint.Render(right), width)
}

// sectionHead is that head over the whole window, which is what a screen that
// draws a single column uses.
func (m *Model) sectionHead(title, right string) string {
	return columnHead(title, right, m.width)
}
