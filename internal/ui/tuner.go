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

// tunerRows is the meter of the tuner, drawn under its head on the config
// screen and given whatever room is left there. It has no keys and nothing to
// answer, so it is the last block on a screen somebody is already on rather
// than a screen of its own, and it gives its lines up from the bottom when the
// window is too short for all of them.
func (m *Model) tunerRows(room int) []string {
	if room < 1 {
		return nil
	}

	width := m.width - 8
	if width > 61 {
		width = 61
	}
	if width < 21 {
		width = 21
	}

	quiet := m.quiet()
	indent := strings.Repeat(" ", 3)

	rows := []string{indent + m.needle(width, quiet)}
	if room >= 3 {
		rows = append(rows, indent+styleFaint.Render(scaleLabels(width)))
	}
	if room >= 4 {
		rows = append(rows, "")
	}
	if room >= 2 {
		rows = append(rows, indent+m.strings(width))
	}

	// the blank under the head is what every other section on that screen has,
	// and it is the first line given up when the window is short
	if room > len(rows) {
		rows = append([]string{""}, rows...)
	}

	return rows
}

// tunerHead is what the section head carries on its right: the note being
// heard, how far off it is and which way the peg turns. It is the whole of the
// tuner on a window with no room for the meter under it.
func (m *Model) tunerHead() string {
	if m.quiet() {
		return "play one string on its own"
	}
	return fmt.Sprintf("%s  %.2f Hz   %s", m.level.Name, m.level.Freq, m.reading())
}

// quiet is a room with nothing being played in it, which is what the tuner
// opens on and goes back to.
func (m *Model) quiet() bool {
	return m.level.Freq == 0 || time.Since(m.silence) > 3*time.Second
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

	if math.Abs(m.level.Cents) <= inTune {
		return styleOk.Render("\u2713  in tune")
	}

	return styleWarn.Render(fmt.Sprintf("%+.0f cents", m.level.Cents)) +
		styleSubtle.Render("  "+m.turn())
}

// reading is the verdict in one phrase, for the head of the section.
func (m *Model) reading() string {
	if math.Abs(m.level.Cents) <= inTune {
		return "\u2713 in tune"
	}
	return fmt.Sprintf("%+.0f cents, %s", m.level.Cents, m.turn())
}

// turn is which way the peg goes, since a number of cents alone does not say.
func (m *Model) turn() string {
	if m.level.Cents < 0 {
		return "flat, tune up"
	}
	return "sharp, tune down"
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
	if !m.quiet() {
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
