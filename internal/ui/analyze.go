package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewAnalyze() string {
	if m.asking == askingRecording {
		return m.viewAsk("Listen to a recording",
			"Any wav, flac, ogg or mp3 of you playing the song straight through.\n"+
				"It runs through the same detector the practice screen uses, so the\n"+
				"two agree on what you played.")
	}
	if m.running {
		return m.viewWorking()
	}
	if m.report == nil {
		return m.viewNoReport()
	}

	return m.viewReport()
}

func (m *Model) viewNoReport() string {
	name := "no song picked yet"
	if m.current != nil {
		name = m.current.Title
	}

	lines := []string{
		"",
		m.sectionHead("ANALYZE A RECORDING", name),
		"",
		"  " + styleSubtle.Render("Play the song into a recorder, then let it be marked here."),
		"  " + styleSubtle.Render("Press "+styleAccent.Render("a")+styleSubtle.Render(" and give it the file.")),
		"",
		"  " + styleFaint.Render("It reports the notes you missed, the ones you played instead,"),
		"  " + styleFaint.Render("and how fast the take actually was."),
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewWorking() string {
	lines := []string{
		"",
		m.sectionHead("LISTENING", fmt.Sprintf("%.0f%%", m.progress*100)),
		"",
		"  " + bar(m.progress, m.width-4, styleAccent),
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewReport() string {
	summary := m.report.Summary
	style := styleOk
	switch {
	case summary.Accuracy < 0.6:
		style = styleBad
	case summary.Accuracy < 0.85:
		style = styleWarn
	}

	lines := []string{
		"",
		pad("  "+styleHeading.Render(m.report.Song),
			style.Render(fmt.Sprintf("%.0f%%", summary.Accuracy*100))+
				styleFaint.Render(fmt.Sprintf("  of %d notes", summary.Notes)), m.width),
		"",
		"  " + m.counts(summary),
		"",
		"  " + styleAccent.Render("BY MEASURE"),
		"",
	}

	lines = append(lines, m.measureRows()...)
	lines = append(lines, "", "  "+styleAccent.Render("WHAT TO FIX"), "")
	lines = append(lines, m.problemRows(m.space()-len(lines)-2)...)

	return strings.Join(lines, "\n")
}

func (m *Model) counts(summary reportSummary) string {
	parts := []string{
		styleOk.Render(fmt.Sprint(summary.Hits)) + styleFaint.Render(" right"),
		styleBad.Render(fmt.Sprint(summary.Notes-summary.Hits-summary.Missed)) + styleFaint.Render(" wrong"),
		styleWarn.Render(fmt.Sprint(summary.Missed)) + styleFaint.Render(" missed"),
		styleSubtle.Render(fmt.Sprint(summary.Extras)) + styleFaint.Render(" extra"),
	}

	line := strings.Join(parts, styleFaint.Render("    "))
	if summary.Tempo > 0 {
		line += styleFaint.Render("    played at ") + styleSubtle.Render(fmt.Sprintf("♩%.0f", summary.Tempo))
	}

	return line
}

// measureRows is the small bar chart that says where the trouble is. Reading
// four hundred verdicts one by one says nothing; one row per measure says the
// second half of the solo is the part to go back to.
func (m *Model) measureRows() []string {
	const cells = 12

	var rows []string
	line := "  "
	for index, measure := range m.report.Measures {
		share := 0.0
		if measure.Notes > 0 {
			share = float64(measure.Hits) / float64(measure.Notes)
		}

		style := styleOk
		switch {
		case share < 0.6:
			style = styleBad
		case share < 0.85:
			style = styleWarn
		}

		block := styleFaint.Render(fmt.Sprintf("%3d ", measure.Index)) +
			style.Render(strings.Repeat("▆", int(share*cells))) +
			styleFaint.Render(strings.Repeat("▁", cells-int(share*cells)))

		line += block + "   "
		if (index+1)%4 == 0 {
			rows = append(rows, strings.TrimRight(line, " "))
			line = "  "
			if len(rows) >= 6 {
				break
			}
		}
	}

	if strings.TrimSpace(line) != "" {
		rows = append(rows, strings.TrimRight(line, " "))
	}

	return rows
}

// problemRows lists only what went wrong. A list that also holds every note
// that was right is a list nobody scrolls to the end of.
func (m *Model) problemRows(room int) []string {
	if room < 1 {
		return nil
	}

	var problems []reportNote
	for _, note := range m.report.Notes {
		if note.Kind != "hit" {
			problems = append(problems, note)
		}
	}

	if len(problems) == 0 {
		return []string{"  " + styleOk.Render("nothing. every note landed.")}
	}

	start := m.reportRow
	if start > len(problems)-1 {
		start = len(problems) - 1
	}
	end := start + room
	if end > len(problems) {
		end = len(problems)
	}

	rows := make([]string, 0, end-start)
	for _, note := range problems[start:end] {
		rows = append(rows, m.problemRow(note))
	}

	if end < len(problems) {
		rows[len(rows)-1] = styleFaint.Render(fmt.Sprintf("  %d more", len(problems)-end+1))
	}

	return rows
}

// position writes a note the way a tab writes it inline, string letter then
// fret. Printing the string as a number reads as a fraction and nobody can
// tell which half is which.
func (m *Model) position(str, fret int) string {
	letter := fmt.Sprint(str)
	if m.current != nil {
		letter = m.current.Label(str)
	}
	return fmt.Sprintf("%s|%d", letter, fret)
}

func (m *Model) problemRow(note reportNote) string {
	where := styleFaint.Render(fmt.Sprintf("  m%-4d", note.Measure))

	var frets []string
	for _, fret := range note.Frets {
		frets = append(frets, m.position(fret[0], fret[1]))
	}

	expected := styleSubtle.Render(strings.Join(note.Expected, " ")) +
		styleFaint.Render("  "+strings.Join(frets, " "))

	verdict := styleWarn.Render("not played")
	if note.Kind == "wrong" {
		verdict = styleBad.Render("played " + note.Played)
	}
	if note.Kind == "extra" {
		expected = styleFaint.Render("nothing written here")
		verdict = styleSubtle.Render("played " + note.Played)
	}

	left := where + expected
	return pad(truncate(left, m.width-lipgloss.Width(verdict)-4), verdict+"  ", m.width)
}
