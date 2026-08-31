package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// The same song the tab draws, seen from the other end: the notes come at you
// and the line at the bottom is now. It is a way of drawing what the engine
// already knows, so nothing here decides anything about the music.

// lane is the width of one string on the highway. Four cells is what two fret
// digits need with a space either side of them.
const lane = 4

// lookahead is how far ahead the top of the screen is. Two and a half seconds
// is about a bar at a hundred, which is enough warning to move a hand and not
// so much that the notes crawl.
const lookahead = 2.5 * float64(time.Second)

func (m *Model) viewHighway(rows int) []string {
	if rows < 4 {
		rows = 4
	}

	tuning := m.current.Tuning
	indent := (m.width - len(tuning)*lane) / 2
	if indent < 2 {
		indent = 2
	}

	// the low string is on the left, the way a guitar hero board is laid out
	// and the way a guitar looks from where the player is standing
	order := make([]song.String, 0, len(tuning))
	for index := len(tuning) - 1; index >= 0; index-- {
		order = append(order, tuning[index])
	}

	reference := m.reference()
	perRow := lookahead / float64(rows)

	grid := make([][]string, rows+1)
	for row := range grid {
		grid[row] = make([]string, len(order))
	}

	for _, event := range m.upcoming(reference, rows, perRow) {
		offset := float64(time.Duration(event.Time*float64(time.Second))) - reference
		row := rows - int(offset/perRow)
		if row < 0 || row > rows {
			continue
		}
		for _, note := range event.Notes {
			for at, str := range order {
				if str.Number == note.String {
					grid[row][at] = strconv.Itoa(note.Fret)
				}
			}
		}
	}

	lines := []string{m.laneLabels(order, indent)}
	for row := 0; row < rows; row++ {
		lines = append(lines, m.laneRow(grid[row], order, indent, m.gridline(reference, rows, perRow, row)))
	}

	return append(lines, m.hitLine(grid[rows], order, indent))
}

// reference is what the bottom of the highway means. With the clock running it
// is the clock; standing still it is the note being waited for, which puts that
// note on the line and the ones after it in the air above it.
func (m *Model) reference() float64 {
	if m.engine.Mode == practice.Tempo && m.engine.Running() {
		return float64(m.engine.Elapsed(time.Now()))
	}

	if event, playing := m.engine.Current(); playing {
		return event.Time * float64(time.Second)
	}

	return 0
}

func (m *Model) upcoming(reference float64, rows int, perRow float64) []song.Event {
	var events []song.Event
	for _, event := range m.engine.Events {
		offset := event.Time*float64(time.Second) - reference
		if offset < -perRow/2 {
			continue
		}
		if offset > float64(rows)*perRow {
			break
		}
		events = append(events, event)
	}
	return events
}

func (m *Model) laneLabels(order []song.String, indent int) string {
	line := strings.Repeat(" ", indent)
	for _, str := range order {
		line += center(styleString.Render(m.current.Label(str.Number)), lane, m.current.Label(str.Number))
	}
	return line
}

func (m *Model) laneRow(cells []string, order []song.String, indent int, mark string) string {
	line := strings.Repeat(" ", indent)

	for at := range order {
		if cells[at] == "" {
			line += center(styleFaint.Render(mark), lane, mark)
			continue
		}
		line += center(styleTabHere.Render(cells[at]), lane, cells[at])
	}

	return line
}

// gridline says what an empty lane is drawn with on this row. A plain pipe
// everywhere gives no sense of speed at all: the beat crossings are what turn
// the board into a rhythm and not a list.
func (m *Model) gridline(reference float64, rows int, perRow float64, row int) string {
	// the row covers the time between itself and the row under it, and a beat
	// falling anywhere in there belongs to it
	top := (reference + float64(rows-row)*perRow) / float64(time.Second)
	bottom := (reference + float64(rows-row-1)*perRow) / float64(time.Second)

	high, low := m.current.BeatAt(top), m.current.BeatAt(bottom)
	if int(high) == int(low) {
		return "│"
	}

	// a bar line is worth more than a beat line, and the two have to be told
	// apart at a glance or neither is worth drawing
	measure := m.current.MeasureAt(top)
	if int(high) == int(measure.Beat) && measure.Beat > 0 {
		return "┿"
	}

	return "┼"
}

// hitLine is now. It is drawn through rather than under the lanes, so a note
// sitting on it is plainly on it and not merely near it.
func (m *Model) hitLine(cells []string, order []song.String, indent int) string {
	line := styleAccent.Render(strings.Repeat("═", indent))

	for at := range order {
		if cells[at] == "" {
			line += fill(styleAccent.Render("╪"), "╪", "═", styleAccent)
			continue
		}
		line += fill(styleAccent.Bold(true).Render(cells[at]), cells[at], "═", styleAccent)
	}

	return line + styleAccent.Render("══")
}

// center puts text in the middle of a lane. The styled string and the plain one
// are both passed because the escapes make the styled one the wrong length to
// measure.
func center(styled string, width int, plain string) string {
	return pack(styled, plain, width, " ", lipgloss.NewStyle())
}

// fill is center with something other than a space around it, which is what
// makes the hit line continuous.
func fill(styled, plain, filler string, style lipgloss.Style) string {
	return pack(styled, plain, lane, filler, style)
}

func pack(styled, plain string, width int, filler string, style lipgloss.Style) string {
	left := (width - len([]rune(plain))) / 2
	right := width - len([]rune(plain)) - left
	if left < 0 {
		left, right = 0, 0
	}
	return style.Render(strings.Repeat(filler, left)) + styled + style.Render(strings.Repeat(filler, right))
}

// streak is how many notes have landed in a row. It is the one number a
// scrolling board is read for, and it is worth more on the screen than an
// accuracy that moves by a fraction of a percent per note.
func (m *Model) streak() int {
	count := 0
	for index := m.engine.Cursor() - 1; index >= 0; index-- {
		if m.engine.Result(index).Verdict != practice.Hit {
			break
		}
		count++
	}
	return count
}

func (m *Model) highwayFoot() string {
	streak := m.streak()
	if streak == 0 {
		return ""
	}

	style := styleSubtle
	if streak >= 10 {
		style = styleOk
	}

	return "  " + style.Render(fmt.Sprintf("%d in a row", streak))
}
