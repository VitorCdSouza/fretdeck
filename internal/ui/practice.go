package ui

import (
	"fmt"
	"sort"
	"strconv"
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

	lines := []string{"", m.practiceHead(), ""}

	if m.highway {
		// the board takes what is left after the head, the callout and the
		// progress line, since a taller board is a longer warning
		lines = append(lines, m.viewHighway(m.space()-len(lines)-5)...)
		lines = append(lines, "", m.callout()+m.highwayFoot())
		filler := m.space() - len(lines) - 1
		return strings.Join(lines, "\n") + blank(filler) + "\n" + m.progressLine()
	}

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

	view := styleFaint.Render("tab")
	if m.highway {
		view = styleFaint.Render("highway")
	}

	mode := view + styleFaint.Render("   ") + styleAccent.Render(m.engine.Mode.String())
	if m.current.Untimed {
		mode = view + styleFaint.Render("   ") + styleFaint.Render("no rhythm")
	}
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
	if m.engine.Repeats(m.measure()) {
		measure = styleAccent.Render(fmt.Sprintf("measure %d", m.measure()))
	}

	parts := []string{mode}
	if beat := m.beatHead(); beat != "" {
		parts = append(parts, beat)
	}
	parts = append(parts, styleFaint.Render(m.engine.Level().String()))
	if repeat := m.repeatHead(); repeat != "" {
		parts = append(parts, repeat)
	}
	parts = append(parts, measure, accuracy)

	right := strings.Join(parts, styleFaint.Render("   "))
	left := truncate("  "+title, m.width-lipgloss.Width(right)-2)

	return pad(left, right, m.width)
}

// beatHead is the beat being counted, and it is only on the screen where there
// is one: a tab with no rhythm and no click is played to nothing at all.
func (m *Model) beatHead() string {
	if m.engine.Mode != practice.Tempo && !m.engine.ClickOn() {
		return ""
	}

	text := fmt.Sprintf("%.0f bpm", m.engine.Bpm())
	if m.engine.ClickOn() {
		return styleAccent.Render(text)
	}
	return styleFaint.Render(text)
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
	m.engine.Loop(time.Now())

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
	if !m.engine.Done() && m.engine.Result(m.engine.Cursor()).Verdict == practice.Wrong {
		here = styleBad
	}

	label := styleString.Render(row.Label)
	if row.At != "" {
		label = here.Render(row.Label)
	}

	return "  " + label + "  " +
		styleTabPast.Render(row.Before) + here.Render(row.At) + styleTabNext.Render(row.After)
}

// callout is the line that tells you what to play and what you played. It is
// the only part of the screen somebody looks at while their hands are busy, so
// it says the note by name and by where it is on the neck.
func (m *Model) callout() string {
	// the app has one text field and it is drawn where it was opened. asking
	// for a beat here and typing it into the music screen would be typing into
	// a line nobody on this screen can see
	if m.input.Focused() {
		return styleAccent.Render("  ▸") + m.input.View()
	}

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
	metronome := m.metronome()
	width := m.width - 4 - lipgloss.Width(metronome)

	return "  " + metronome + bar(m.engine.Progress(), width, styleAccent)
}

// metronome counts the bar on the screen, and out loud as well when the click
// was asked for. The count is always drawn: the input is open while it sounds,
// so the click is off until somebody turns it on and the dots are what is left
// for the eye when it is.
func (m *Model) metronome() string {
	running := m.engine.Mode == practice.Tempo && m.engine.Running()
	if !running && !m.engine.ClickOn() {
		return ""
	}

	now := time.Now()
	pulse, count := m.engine.ClickPulse(now)
	counting := m.engine.CountIn(now) > 0

	dots := make([]string, count)
	for index := range dots {
		switch {
		case counting:
			dots[index] = styleFaint.Render("○")
		case index == pulse:
			dots[index] = styleAccent.Render("●")
		default:
			dots[index] = styleFaint.Render("○")
		}
	}

	return strings.Join(dots, " ") + "   "
}

// sounding is the notes of an event from the lowest up. They arrive in the
// order the file wrote them, which is neither the order the strings are in nor
// the order somebody would call them out.
func sounding(event song.Event) []song.Note {
	notes := append([]song.Note(nil), event.Notes...)
	sort.SliceStable(notes, func(a, b int) bool { return notes[a].Midi < notes[b].Midi })
	return notes
}

// chordName is what to call the event out loud. One note is its name, two are
// both names, and anything thicker is the lowest note and how many ring over
// it. The count says "notes" because a bare +2 beside a note name reads as two
// semitones.
func chordName(event song.Event) string {
	notes := sounding(event)
	switch len(notes) {
	case 0:
		return ""
	case 1:
		return song.NoteName(notes[0].Midi)
	case 2:
		return song.NoteName(notes[0].Midi) + " " + song.NoteName(notes[1].Midi)
	}

	return fmt.Sprintf("%s +%d notes", song.NoteName(notes[0].Midi), len(notes)-1)
}

// where says the position on the neck, which is the part somebody with a
// guitar in their hands can act on without translating anything. It runs from
// the lowest note up, in the order the names beside it are written.
func where(event song.Event) string {
	notes := sounding(event)
	parts := make([]string, 0, len(notes))
	for _, note := range notes {
		// the technique is the half of it the fret number cannot say, and it is
		// only there when the level asks for it
		how := ""
		if note.Technique != "" {
			how = " " + string(note.Technique)
		}

		if note.Fret == 0 {
			parts = append(parts, fmt.Sprintf("string %d open%s", note.String, how))
			continue
		}
		parts = append(parts, fmt.Sprintf("string %d fret %d%s", note.String, note.Fret, how))
	}

	if len(parts) > 3 {
		return strings.Join(parts[:3], " · ") + " …"
	}
	return strings.Join(parts, " · ")
}
