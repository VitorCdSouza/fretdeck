package ui

import (
	"strconv"
	"strings"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// The neck drawn under the tab. A tab says which fret, and somebody who has
// been playing a month still has to count lines to find it; the neck says
// where the finger goes without counting anything.

// span is how many frets are drawn. Twelve is one octave and covers most of
// what a song stays inside, and the window slides when it does not.
const span = 12

// cell is the width of one fret, wire not included.
const cell = 3

// inlays are the frets a guitar has a dot on. They are the only landmark on a
// neck, and a drawing without them is a ladder.
var inlays = map[int]string{3: "·", 5: "·", 7: "·", 9: "·", 12: ":"}

// fretboard draws the neck with the notes of the event pressed on it.
func (m *Model) fretboard(event song.Event, tuning []song.String) []string {
	first := 1
	for _, note := range event.Notes {
		if note.Fret > span {
			if start := note.Fret - span + 1; start > first {
				first = start
			}
		}
	}

	pressed := make(map[int]int, len(event.Notes))
	for _, note := range event.Notes {
		pressed[note.String] = note.Fret
	}

	rows := []string{m.fretNumbers(first)}
	for _, str := range tuning {
		rows = append(rows, m.neckLine(str, pressed, first))
	}

	return append(rows, m.inlayRow(first))
}

func (m *Model) neckLine(str song.String, pressed map[int]int, first int) string {
	fret, plays := pressed[str.Number]

	open := " "
	if plays && fret == 0 {
		open = styleAccent.Render("○")
	}

	// the neck uses the same two weights the tab does, so a wound string is
	// the same line on both halves of the screen
	wire := "─"
	if m.tuningOf().Wound(str.Number) {
		wire = "━"
	}

	nut := "╎"
	if first == 1 {
		// the nut is a thicker line than a fret wire, and it is the only way
		// to tell a neck drawn from the first fret from one drawn from the
		// fifth
		nut = "║"
	}

	line := "  " + styleString.Render(str.Label(m.tuningOf())) + " " + open + styleFaint.Render(nut)

	for at := first; at < first+span; at++ {
		if plays && fret == at {
			line += styleFaint.Render(wire) + styleAccent.Render("●") + styleFaint.Render(wire)
		} else {
			line += styleFaint.Render(strings.Repeat(wire, cell))
		}
		line += styleFaint.Render("┼")
	}

	return line
}

func (m *Model) fretNumbers(first int) string {
	line := []rune(strings.Repeat(" ", 5+span*(cell+1)))

	for at := first; at < first+span; at++ {
		if _, marked := inlays[at]; !marked && at != first {
			continue
		}
		text := strconv.Itoa(at)
		offset := 5 + (at-first)*(cell+1) + (cell-len(text))/2
		for index, digit := range text {
			if offset+index < len(line) {
				line[offset+index] = digit
			}
		}
	}

	return styleFaint.Render(strings.TrimRight(string(line), " "))
}

func (m *Model) inlayRow(first int) string {
	line := []rune(strings.Repeat(" ", 5+span*(cell+1)))

	for at := first; at < first+span; at++ {
		dot, marked := inlays[at]
		if !marked {
			continue
		}
		line[5+(at-first)*(cell+1)+1] = []rune(dot)[0]
	}

	return styleFaint.Render(strings.TrimRight(string(line), " "))
}

// tuningOf hands the neck the song whose labels it should use, so a drop tuning
// is drawn with the letters it really has.
func (m *Model) tuningOf() *song.Song {
	if m.current != nil {
		return m.current
	}
	return &song.Song{Tuning: standard}
}
