package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// inTune is how close counts as right. Five cents is under what an ear picks
// out on a single string, and holding a guitar to less than that is a fight
// with the tuning pegs rather than with the tuning.
const inTune = 5.0

func (m *Model) viewTuner() string {
	width := m.width - 8
	if width > 61 {
		width = 61
	}
	if width < 21 {
		width = 21
	}

	quiet := m.level.Freq == 0 || time.Since(m.silence) > 3*time.Second
	indent := strings.Repeat(" ", (m.width-width)/2)

	name, hertz := styleFaint.Render("—"), ""
	if !quiet {
		name = styleHeading.Render(m.level.Name)
		hertz = styleSubtle.Render(fmt.Sprintf("%.2f Hz", m.level.Freq))
	}

	// the tuner has one thing on it and a screen to put it on, so it sits in
	// the middle instead of hanging off the top edge
	lines := []string{
		"",
		indent + pad(name, hertz, width),
		"",
		indent + m.needle(width, quiet),
		indent + styleFaint.Render(scaleLabels(width)),
		"",
		indent + m.verdict(quiet),
		"",
		"",
		indent + m.strings(width),
	}

	lead := (m.space() - len(lines)) / 2
	if lead < 1 {
		lead = 1
	}

	return blank(lead) + strings.Join(lines, "\n") + blank(m.space()-len(lines)-lead)
}

// needle draws the deviation as a position on a line, because a number alone
// does not say which way to turn the peg.
func (m *Model) needle(width int, quiet bool) string {
	center := width / 2
	cells := make([]string, width)

	for index := range cells {
		cells[index] = styleFaint.Render("─")
	}
	cells[center] = styleSubtle.Render("┼")

	if !quiet {
		cents := math.Max(-50, math.Min(50, m.level.Cents))
		position := center + int(math.Round(cents/50*float64(center)))
		style := styleWarn
		if math.Abs(m.level.Cents) <= inTune {
			style = styleOk
		}
		cells[position] = style.Render("●")
	}

	return styleFaint.Render("♭ ") + strings.Join(cells, "") + styleFaint.Render(" ♯")
}

func scaleLabels(width int) string {
	line := []rune(strings.Repeat(" ", width+4))
	stamp := func(at int, text string) {
		for index, r := range text {
			if at+index >= 0 && at+index < len(line) {
				line[at+index] = r
			}
		}
	}

	stamp(2, "-50")
	stamp(2+width/2, "0")
	stamp(width-1, "+50")

	return strings.TrimRight(string(line), " ")
}

func (m *Model) verdict(quiet bool) string {
	if quiet {
		return styleFaint.Render("play one string on its own")
	}

	cents := m.level.Cents
	if math.Abs(cents) <= inTune {
		return styleOk.Render("✓  in tune")
	}

	direction := "sharp, tune down"
	if cents < 0 {
		direction = "flat, tune up"
	}

	return styleWarn.Render(fmt.Sprintf("%+.0f cents", cents)) + styleSubtle.Render("  "+direction)
}

// strings is the row of open notes of whatever is loaded, with the one being
// played underlined. It answers the question a tuner is usually opened for,
// which is which string is out and not which pitch is in the room.
func (m *Model) strings(width int) string {
	// with no song open the strings are the ones of the instrument answered for
	tuning := m.instrument().Tuning
	if m.current != nil && len(m.current.Tuning) > 0 {
		tuning = m.current.Tuning
	}

	nearest, best := -1, 4
	if m.level.Freq > 0 && time.Since(m.silence) < 3*time.Second {
		for index, str := range tuning {
			if distance := abs(str.Midi - m.level.Midi); distance < best {
				nearest, best = index, distance
			}
		}
	}

	cells := make([]string, 0, len(tuning))
	for index := len(tuning) - 1; index >= 0; index-- {
		label := song.NoteName(tuning[index].Midi)
		if tuning[index].Number == 1 {
			label = strings.ToLower(label)
		}
		if index == nearest {
			cells = append(cells, styleAccent.Bold(true).Render(label))
			continue
		}
		cells = append(cells, styleFaint.Render(label))
	}

	row := strings.Join(cells, styleFaint.Render("   "))
	gap := (width - lipgloss.Width(row)) / 2
	if gap < 0 {
		gap = 0
	}

	return strings.Repeat(" ", gap) + row
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
