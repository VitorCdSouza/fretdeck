package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// tabIndent is the room the string letter takes at the left of every line, so
// the marker over the tab lines up with the column under it.
const tabIndent = 5

func (m *Model) viewPractice() string {
	if m.engine == nil || m.tab == nil {
		return "\n" + styleSubtle.Render("  Pick a song on the library screen.") + blank(m.space()-2)
	}

	width := m.width - tabIndent - 2
	if width < 10 {
		width = 10
	}

	view := m.tab.View(m.engine.Cursor(), width)
	lines := []string{"", m.practiceHead(), ""}
	lines = append(lines, m.marker(view), styleFaint.Render(strings.Repeat(" ", tabIndent)+view.Header))

	for _, row := range view.Rows {
		lines = append(lines, m.tabRow(row))
	}

	lines = append(lines, "", m.callout())

	// the neck only goes on the screen when there is room for all of it. half
	// a neck is worse than none, since the fret it is missing is the one being
	// asked for
	if event, playing := m.engine.Current(); playing && m.space() >= len(lines)+11 {
		lines = append(lines, "")
		lines = append(lines, m.fretboard(event, m.current.Tuning)...)
	}

	// the progress line is pinned to the bottom of the body instead of
	// floating under the tab, so the block above it does not move when the
	// callout grows a second line
	filler := m.space() - len(lines) - 1
	return strings.Join(lines, "\n") + blank(filler) + "\n" + m.progressLine()
}

func (m *Model) practiceHead() string {
	title := styleHeading.Render(m.current.Title)
	if m.current.Track != "" {
		title += styleFaint.Render("  ·  ") + styleSubtle.Render(m.current.Track)
	}

	mode := styleAccent.Render(m.engine.Mode.String())
	if m.engine.Mode == practice.Tempo {
		mode += styleFaint.Render(fmt.Sprintf("  %.0f%%", m.engine.Speed*100))
	}

	score := m.engine.Score()
	accuracy := styleFaint.Render("—")
	if score.Total > 0 {
		style := styleOk
		switch {
		case score.Accuracy < 0.6:
			style = styleBad
		case score.Accuracy < 0.85:
			style = styleWarn
		}
		accuracy = style.Render(fmt.Sprintf("%3.0f%%", score.Accuracy*100))
	}

	measure := styleSubtle.Render(fmt.Sprintf("measure %d", m.measure()))

	return pad("  "+title, strings.Join([]string{mode, measure, accuracy}, styleFaint.Render("   ")), m.width)
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

// tabRow paints one string. What is behind the cursor is dimmed, what is under
// it takes the colour of the verdict, and what is coming stays readable.
func (m *Model) tabRow(row song.Row) string {
	here := styleTabHere
	if !m.engine.Done() && m.engine.Result(m.engine.Cursor()).Verdict == practice.Wrong {
		here = styleBad
	}

	return "  " + styleString.Render(row.Label) + "  " +
		styleTabPast.Render(row.Before) + here.Render(row.At) + styleTabNext.Render(row.After)
}

// callout is the line that tells you what to play and what you played. It is
// the only part of the screen somebody looks at while their hands are busy, so
// it says the note by name and by where it is on the neck.
func (m *Model) callout() string {
	if m.engine.Done() {
		score := m.engine.Score()
		return "  " + styleOk.Render("done") + styleFaint.Render("   ") +
			styleSubtle.Render(fmt.Sprintf("%d of %d right", score.Hits, score.Total))
	}

	if countIn := m.engine.CountIn(time.Now()); countIn > 0 {
		return "  " + styleAccent.Render(fmt.Sprintf("starting in %.0f", countIn.Seconds()+0.99))
	}

	event, _ := m.engine.Current()
	left := "  " + styleFaint.Render("play  ") + styleAccent.Render(chordName(event)) +
		styleFaint.Render("   ") + styleSubtle.Render(where(event))

	if m.heard.Name == "" || time.Since(m.silence) > 3*time.Second {
		return left
	}

	verdict := m.engine.Result(m.engine.Cursor())
	right := styleFaint.Render("heard  ")
	switch verdict.Verdict {
	case practice.Wrong:
		right += styleBad.Render(m.heard.Name + "  ✗")
	default:
		right += styleSubtle.Render(m.heard.Name)
	}

	// the note just before the cursor is the one that was accepted, and saying
	// so is the only confirmation the tempo mode gives while it runs
	if m.engine.Cursor() > 0 && m.engine.Result(m.engine.Cursor()-1).Verdict == practice.Hit {
		right = styleFaint.Render("heard  ") + styleOk.Render(m.heard.Name+"  ✓")
	}

	return pad(left, right+"  ", m.width)
}

func (m *Model) progressLine() string {
	width := m.width - 4
	line := "  " + bar(m.engine.Progress(), width, styleAccent)

	if m.engine.Mode != practice.Tempo || !m.engine.Running() {
		return line
	}

	// the metronome is a beat count, not a sound: nothing here can make noise
	// while the microphone is open without being heard as a note
	beat := int(m.engine.Beat(time.Now()))
	if beat != m.beat {
		m.beat = beat
	}

	return line
}

// chordName is what to call the event out loud. One note is its name, a chord
// is its lowest note and how many strings ring with it.
func chordName(event song.Event) string {
	if len(event.Notes) == 0 {
		return ""
	}

	name := song.NoteName(event.Notes[0].Midi)
	if len(event.Notes) == 1 {
		return name
	}

	return fmt.Sprintf("%s +%d", name, len(event.Notes)-1)
}

// where says the position on the neck, which is the part somebody with a
// guitar in their hands can act on without translating anything.
func where(event song.Event) string {
	parts := make([]string, 0, len(event.Notes))
	for _, note := range event.Notes {
		if note.Fret == 0 {
			parts = append(parts, fmt.Sprintf("string %d open", note.String))
			continue
		}
		parts = append(parts, fmt.Sprintf("string %d fret %d", note.String, note.Fret))
	}

	if len(parts) > 3 {
		return strings.Join(parts[:3], " · ") + " …"
	}
	return strings.Join(parts, " · ")
}
