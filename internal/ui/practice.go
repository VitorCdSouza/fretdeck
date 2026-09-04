package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// The practice screen is the tab and the cursor on it, and nothing else. What
// moves the cursor is a key or the note being played right; what the screen
// says besides the tab is which measure is under the cursor and which measures
// are being repeated.

// tabIndent is the room the string letter takes at the left of every line, so
// the marker over the tab lines up with the column under it.
const tabIndent = 5

func (m *Model) viewPractice() string {
	if m.engine == nil || m.tab == nil {
		return "\n" + styleSubtle.Render("  Pick a song on the library screen.") + blank(m.space()-2)
	}

	lines := []string{"", m.practiceHead(), ""}

	width := m.width - tabIndent - 2
	if width < 10 {
		width = 10
	}

	view := m.tab.View(m.engine.Cursor(), width)
	lines = append(lines, m.marker(view), styleFaint.Render(strings.Repeat(" ", tabIndent)+view.Header))

	for _, row := range view.Rows {
		lines = append(lines, m.tabRow(row))
	}

	if band := m.repeatBand(view); band != "" {
		lines = append(lines, band)
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) practiceHead() string {
	title := styleHeading.Render(m.current.Title)
	if m.current.Track != "" {
		title += styleFaint.Render("  ·  ") + styleSubtle.Render(m.current.Track)
	}

	// the number over the bar line says which measure the cursor is in, so a
	// second one in the corner is the same answer twice
	right := m.repeatHead()
	left := truncate("  "+title, m.width-lipgloss.Width(right)-2)

	return pad(left, right, m.width)
}

// repeatHead is what the passage being looped over is called, and the number
// of times round it has been. The mode says so even with nothing picked yet,
// since a mode with no sign of itself is one nobody knows they are in.
func (m *Model) repeatHead() string {
	if !m.engine.Looping() {
		if m.mode == modeRepeat {
			return styleAccent.Render("repeat")
		}
		return ""
	}

	text := styleAccent.Render("repeat " + measureList(m.engine.Measures()))
	if passes := m.engine.Passes(); passes > 0 {
		text += styleFaint.Render(fmt.Sprintf("  ×%d", passes))
	}
	return text
}

// pickRepeat marks the measure under the cursor, or lets it go. The loop is
// closed as it is marked, so a passage behind the cursor starts repeating at
// once instead of the next time the song runs off its end.
func (m *Model) pickRepeat() {
	m.engine.ToggleRepeat(m.measure())
	m.engine.Loop()

	if !m.engine.Looping() {
		m.status = "nothing is being repeated"
		return
	}
	m.status = "repeating " + measureList(m.engine.Measures())
}

// repeatBand is the line under the tab that says which measures are looped
// over. It is drawn from the spans the tab reports, so the mark cannot drift
// off the columns above it, and it is an empty line while nothing is picked so
// the tab does not jump when the first one is.
func (m *Model) repeatBand(view song.View) string {
	if !m.engine.Looping() && m.mode != modeRepeat {
		return ""
	}

	total := 0
	for _, span := range view.Spans {
		if end := span.At + span.Width; end > total {
			total = end
		}
	}

	line := []rune(strings.Repeat(" ", total))
	for _, span := range view.Spans {
		if !m.engine.Repeats(span.Measure) {
			continue
		}
		for at := span.At; at < span.At+span.Width && at < len(line); at++ {
			line[at] = '━'
		}
	}

	return strings.Repeat(" ", tabIndent) + styleAccent.Render(strings.TrimRight(string(line), " "))
}

// measureList writes the picked measures the way somebody would say them, so
// four bars in a row read as one passage and not as four numbers.
func measureList(measures []int) string {
	parts := make([]string, 0, len(measures))
	for index := 0; index < len(measures); {
		end := index
		for end+1 < len(measures) && measures[end+1] == measures[end]+1 {
			end++
		}
		if end == index {
			parts = append(parts, strconv.Itoa(measures[index]))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", measures[index], measures[end]))
		}
		index = end + 1
	}

	return strings.Join(parts, " ")
}

func (m *Model) measure() int {
	events := m.engine.Events
	cursor := m.engine.Cursor()
	if cursor >= len(events) {
		if len(events) == 0 {
			return 0
		}
		return events[len(events)-1].Measure
	}
	return events[cursor].Measure
}

// marker is the caret above the tab. It points at the column being waited for,
// which is what lets the eye find the place without reading fret numbers.
func (m *Model) marker(view song.View) string {
	if len(view.Rows) == 0 {
		return ""
	}

	offset := tabIndent + lipgloss.Width(view.Rows[0].Before)
	return strings.Repeat(" ", offset) + styleAccent.Render(marker)
}

// tabRow paints one string. What is behind the cursor is dimmed, the fret under
// it takes the colour of the verdict, and what is coming stays readable. The
// string letter lights up with the fret, so which strings the column asks for
// is answered at the left edge without counting lines across the screen.
func (m *Model) tabRow(row song.Row) string {
	here := styleTabHere
	if !m.engine.Empty() && m.engine.Result(m.engine.Cursor()).Verdict == practice.Wrong {
		here = styleBad
	}

	label := styleString.Render(row.Label)
	if row.At != "" {
		label = here.Render(row.Label)
	}

	return "  " + label + "  " +
		styleTabPast.Render(row.Before) + here.Render(row.At) + styleTabNext.Render(row.After)
}
