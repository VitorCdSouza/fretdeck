package song

import (
	"strconv"
	"strings"
)

// CellsPerBeat is how many terminal cells one quarter note takes. A sixteenth
// note lands on a single cell, which is the tightest a tab can be read at.
const CellsPerBeat = 4

// column is one vertical slice of the tab: either a bar line or an attack.
// Width comes from how long the attack lasts, so the drawing carries the
// rhythm and not only the order of the notes.
type column struct {
	event   int
	measure int
	bar     bool
	width   int
	frets   map[int]string
}

// Tab is a song laid out in columns, built once and then sliced by the screen.
type Tab struct {
	song    *Song
	columns []column
	byEvent []int
}

// Row is one string of the tab cut in three, so the caller can paint what is
// behind the cursor, the cursor itself and what is coming in three colours.
type Row struct {
	Label  string
	Before string
	At     string
	After  string
}

// View is what fits on the screen around the current event.
type View struct {
	Rows    []Row
	Header  string
	Measure int
}

func NewTab(s *Song, events []Event) *Tab {
	tab := &Tab{song: s, byEvent: make([]int, len(events))}
	measure := 0

	for index, event := range events {
		if event.Measure != measure {
			measure = event.Measure
			tab.columns = append(tab.columns, column{event: -1, measure: measure, bar: true, width: 1})
		}

		frets := make(map[int]string, len(event.Notes))
		widest := 1
		for _, note := range event.Notes {
			text := strconv.Itoa(note.Fret)
			frets[note.String] = text
			if len(text) > widest {
				widest = len(text)
			}
		}

		// the gap to the next attack is what the note is worth on the page. a
		// rest stretches the column before it, which is how a tab shows one
		width := widest + 1
		if index+1 < len(events) {
			if beats := events[index+1].Beat - event.Beat; beats > 0 {
				if scaled := int(beats * CellsPerBeat); scaled > width {
					width = scaled
				}
			}
		}

		tab.byEvent[index] = len(tab.columns)
		tab.columns = append(tab.columns, column{
			event:   index,
			measure: event.Measure,
			width:   width,
			frets:   frets,
		})
	}

	return tab
}

func (t *Tab) fill(number int) rune {
	if t.song.Wound(number) {
		return '━'
	}
	return '─'
}

// cell draws one column on one string, the fret text followed by the line.
func (t *Tab) cell(col column, number int) string {
	if col.bar {
		return "│"
	}

	fill := string(t.fill(number))
	text, played := col.frets[number]
	if !played {
		return strings.Repeat(fill, col.width)
	}

	return text + strings.Repeat(fill, col.width-len(text))
}

// window picks the first and last column to draw so the cursor sits a third of
// the way in, which leaves most of the screen showing what is coming.
func (t *Tab) window(cursor, width int) (int, int) {
	if width < 4 {
		width = 4
	}

	start := cursor
	for behind := 0; start > 0; start-- {
		behind += t.columns[start-1].width
		if behind > width/3 {
			break
		}
	}

	end, total := start, 0
	for end < len(t.columns) {
		total += t.columns[end].width
		if total > width {
			break
		}
		end++
	}

	return start, end
}

// View draws the tab around the given event. A cursor out of range draws the
// tab from the top, which is what an idle screen shows.
func (t *Tab) View(event, width int) View {
	if len(t.columns) == 0 {
		return View{}
	}

	cursor := 0
	if event >= 0 && event < len(t.byEvent) {
		cursor = t.byEvent[event]
	}

	start, end := t.window(cursor, width)
	rows := make([]Row, 0, len(t.song.Tuning))

	for _, str := range t.song.Tuning {
		var before, at, after strings.Builder
		for index := start; index < end; index++ {
			text := t.cell(t.columns[index], str.Number)
			switch {
			case index < cursor:
				before.WriteString(text)
			case index == cursor:
				at.WriteString(text)
			default:
				after.WriteString(text)
			}
		}
		rows = append(rows, Row{
			Label:  t.song.Label(str.Number),
			Before: before.String(),
			At:     at.String(),
			After:  after.String(),
		})
	}

	return View{Rows: rows, Header: t.header(start, end), Measure: t.columns[cursor].measure}
}

// header stamps the measure number over each bar line. It is written into a
// rune slice instead of joined, because a number is wider than the bar it
// belongs to and has to overwrite the spaces after it.
func (t *Tab) header(start, end int) string {
	total := 0
	for index := start; index < end; index++ {
		total += t.columns[index].width
	}

	line := []rune(strings.Repeat(" ", total))
	offset := 0

	for index := start; index < end; index++ {
		col := t.columns[index]
		if col.bar {
			for position, digit := range strconv.Itoa(col.measure) {
				if offset+position < len(line) {
					line[offset+position] = digit
				}
			}
		}
		offset += col.width
	}

	return strings.TrimRight(string(line), " ")
}
