package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/config"
)

// The metronome is the one thing here that makes a sound, and the tone it
// makes is the whole of what lets it. It sits half a semitone off the grid a
// fretboard is written on and above the highest note the detector names, so
// the pitches it can be mistaken for are pitches no string is ever tuned to,
// and pitch.CLICK_PITCHES is what the tracker stays deaf to for as long as the
// click is sounding. Anything else that ever wants to be heard out of the
// speakers pays that same bill.
//
// The count is the worker's. It writes a beat at a time into an output stream
// of its own, which is consumed at exactly the rate the card runs at, and what
// this screen sends is the beat and the bar and nothing else. Nothing is drawn
// beating here on purpose: a second count, running off the frame clock, would
// disagree with the one being heard inside a bar, and the wrong one is the one
// nobody can tell from the right one.

// what the cursor of the metronome screen stands on.
const (
	metroBeat = iota
	metroBar
	metroClick
	metroRows
)

// stepBeat is what the bracket keys move the beat by, since walking from sixty
// to a hundred and forty one at a time is a key held down.
const stepBeat = 10

func (m *Model) keyMetronome(key string) tea.Cmd {
	switch key {
	// the click starts and stops from any row, since it is what the screen is for
	case " ":
		return m.setClick(!m.clicking)

	case "l", "right", "enter":
		return m.stepMetronome(1)
	case "h", "left":
		return m.stepMetronome(-1)

	case "[":
		return m.setBpm(m.cfg.Bpm - stepBeat)
	case "]":
		return m.setBpm(m.cfg.Bpm + stepBeat)
	}

	return nil
}

// stepMetronome is h and l: less and more of whatever the cursor is on, and
// off and on when that is the click itself.
func (m *Model) stepMetronome(delta int) tea.Cmd {
	switch m.metro {
	case metroBeat:
		return m.setBpm(m.cfg.Bpm + float64(delta))
	case metroBar:
		return m.setBeats(m.cfg.Beats + delta)
	case metroClick:
		return m.setClick(delta > 0)
	}
	return nil
}

func (m *Model) setBpm(bpm float64) tea.Cmd {
	if bpm < config.MinBpm {
		bpm = config.MinBpm
	}
	if bpm > config.MaxBpm {
		bpm = config.MaxBpm
	}
	if bpm == m.cfg.Bpm {
		return nil
	}

	m.cfg.Bpm = bpm
	return m.keepMetronome()
}

func (m *Model) setBeats(beats int) tea.Cmd {
	if beats < config.MinBeats {
		beats = config.MinBeats
	}
	if beats > config.MaxBeats {
		beats = config.MaxBeats
	}
	if beats == m.cfg.Beats {
		return nil
	}

	m.cfg.Beats = beats
	return m.keepMetronome()
}

// setClick starts the count or stops it. The worker is told either way, since
// what turns the click off is a beat of nothing, and the screen says which of
// the two it is in the head and on the row itself.
func (m *Model) setClick(on bool) tea.Cmd {
	if on == m.clicking {
		return nil
	}

	m.clicking = on
	return m.click()
}

// keepMetronome writes the beat down and hands the worker the new one, so a
// number changed while it is counting is heard on the next beat.
func (m *Model) keepMetronome() tea.Cmd {
	if err := m.cfg.Save(); err != nil {
		m.fail = err.Error()
		return nil
	}
	if !m.clicking {
		return nil
	}
	return m.click()
}

// clickCommand is the whole of what the worker is told. A bpm of nothing is
// the click turned off, which is why a stop is a command like a start is.
func (m *Model) clickCommand() bridge.Command {
	command := bridge.Command{Action: "click", Beats: m.cfg.Beats}
	if m.clicking {
		command.Bpm = m.cfg.Bpm
	}
	return command
}

func (m *Model) click() tea.Cmd {
	command := m.clickCommand()

	worker := m.worker
	if worker == nil {
		return nil
	}

	return func() tea.Msg {
		_ = worker.Send(command)
		return nil
	}
}

// stopClick is the click going quiet without anybody asking it to, which is a
// worker that died and an output that could not be opened.
func (m *Model) stopClick() {
	m.clicking = false
}

func (m *Model) viewMetronome() string {
	lines := []string{"", m.sectionHead("THE METRONOME", m.metroHead()), ""}

	m.clicks = append(m.clicks, clickable{top: headerLines + len(lines), count: metroRows})
	lines = append(lines,
		m.metroRow(metroBeat, "the beat", fmt.Sprintf("%.0f bpm", m.cfg.Bpm), false),
		m.metroRow(metroBar, "the bar", plural(m.cfg.Beats, "beat"), false),
		m.metroRow(metroClick, "the click", m.clickState(), m.clicking),
	)

	lines = append(lines,
		"",
		"  "+styleSubtle.Render("The count is the worker's own and it goes out of the speakers, so the"),
		"  "+styleSubtle.Render("microphone hears it: the tone is one no string is ever tuned to and the"),
		"  "+styleSubtle.Render("detector stays deaf to it while it sounds."),
		"",
		"  "+styleFaint.Render("It keeps counting on the other screens, which is what practising to it is."),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// metroHead is the whole of the setting on one line, for the section head.
func (m *Model) metroHead() string {
	return fmt.Sprintf("%.0f bpm · %s to the bar · %s", m.cfg.Bpm, plural(m.cfg.Beats, "beat"), m.clickState())
}

func (m *Model) clickState() string {
	if m.clicking {
		return "counting"
	}
	return "off"
}

// metroRow is one row in the shape every list here draws: the mark, what it is
// and what it is set to against the right edge.
func (m *Model) metroRow(row int, name, state string, on bool) string {
	paint := rowPaint(m.metro == row)

	mark, label := "   ", paint.of(styleInk).Render(name)
	if m.metro == row {
		mark, label = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(name)
	}

	value := paint.of(styleFaint).Render(state)
	if on {
		value = paint.of(styleOk).Render(state)
	}

	return paint.pad(mark+label, value+paint.of(styleFaint).Render(" "), m.width)
}
