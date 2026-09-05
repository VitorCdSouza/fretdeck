package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/song"
	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
)

// The config screen asks the two things the app cannot work out on its own:
// what is plugged in, and which input it is plugged into. A first run asks
// them as two steps, one screen each and in that order, and it is the whole of
// the app until they are answered or left for later. An answer that was kept
// is not asked about again, and the second step is the last thing between the
// app and the search screen it opens on from then on.
//
// After that everything is on this one screen, both lists and the switch, with
// one cursor over the lot, because any of it can change.

// configShows says which of the lists are on the screen. The switches are the
// answers that were never asked for, so a first run does not show them.
func (m *Model) configShows() (instruments, inputs bool) {
	switch m.first {
	case firstRunInstrument:
		return true, false
	case firstRunInput:
		return false, true
	}
	return true, true
}

func (m *Model) configCount() int {
	instruments, inputs := m.configShows()

	count := 0
	if instruments {
		count += len(song.Instruments)
	}
	if inputs {
		count += len(m.devices)
	}
	if m.first == firstRunDone {
		count += len(tabsite.Sites) + 1
	}
	return count
}

// what the cursor of the setup screen can be standing on.
type configKind int

const (
	configNothing configKind = iota
	configInstrument
	configInput
	configSite
	configMouse
)

// configPick turns the cursor into what it points at, and where in its own list
// that is.
func (m *Model) configPick() (configKind, int) {
	instruments, inputs := m.configShows()

	row := m.configRow
	if instruments {
		if row < len(song.Instruments) {
			return configInstrument, row
		}
		row -= len(song.Instruments)
	}
	if inputs {
		if row < len(m.devices) {
			return configInput, row
		}
		row -= len(m.devices)
	}
	if m.first == firstRunDone {
		if row < len(tabsite.Sites) {
			return configSite, row
		}
		if row-len(tabsite.Sites) == 0 {
			return configMouse, 0
		}
	}

	return configNothing, 0
}

// deviceOffset is where the input list starts, counted in rows.
func (m *Model) deviceOffset() int {
	if instruments, _ := m.configShows(); instruments {
		return len(song.Instruments)
	}
	return 0
}

// startingRow opens the list on the answer that is in force, rather than at
// the top of it.
func (m *Model) startingRow() {
	if instruments, inputs := m.configShows(); instruments && !inputs {
		for index, item := range song.Instruments {
			if item.Name == m.cfg.Instrument {
				m.configRow = index
				return
			}
		}
		m.configRow = 0
		return
	}

	offset := m.deviceOffset()
	for index, device := range m.devices {
		if m.chosen(device) || (m.cfg.Device < 0 && device.Default) {
			m.configRow = offset + index
			return
		}
	}
	m.configRow = offset
}

// keepConfig is enter: the row under the cursor becomes the answer and is
// written to the config, and on a first run the next question follows.
// step is where the first run has got to, for the line that says so.
func (m *Model) step() string {
	if m.first == firstRunInstrument {
		return "step 1 of 2"
	}
	return "step 2 of 2"
}

// back is h on the second step: the answer to the first one can be changed
// before the run is over.
func (m *Model) back() {
	if m.first != firstRunInput {
		return
	}
	m.first = firstRunInstrument
	m.startingRow()
	m.status = ""
}

// later is esc: the questions go away for this run and neither answer is kept,
// so the next run asks them again. Nothing is listened to in the meantime,
// which the config screen is where to fix.
func (m *Model) later() {
	m.first = firstRunDone
	m.startingRow()
	m.screen = screenMusic
	m.status = "left for later, the config screen has both of them"
}

func (m *Model) keepConfig() tea.Cmd {
	kind, at := m.configPick()

	switch kind {
	case configInstrument:
		chosen := song.Instruments[at]
		m.cfg.Instrument = chosen.Name
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
			return nil
		}
		m.songsterr.Family = m.family()
		m.status = chosen.Name + ", tuned " + chosen.Written()

		if m.first == firstRunInstrument {
			m.first = firstRunInput
			m.startingRow()
		}

		// the difficulty on a playlist is that of the instrument being played
		return m.lookupAgain()

	case configInput:
		chosen := m.devices[at]
		m.cfg.Device, m.cfg.Rate = chosen.Index, chosen.Rate
		m.cfg.Source, m.cfg.Card = chosen.ID, chosen.Card
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
			return nil
		}
		m.status = "input is " + chosen.Name

		if m.first == firstRunInput {
			m.first = firstRunDone
			m.startingRow()
			m.screen = screenMusic
		}
		return m.listen()

	case configSite:
		chosen := tabsite.Sites[at]
		m.cfg.Site = chosen.Name
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
			return nil
		}
		m.site = openSite(chosen.Name)
		m.status = "songs come from " + chosen.Name

		// the rows of the site that was left are pages this one cannot read
		m.results, m.groups, m.pages = nil, nil, nil
		m.found = 0
		if m.query != "" {
			return m.askSite(m.query)
		}
		return nil

	case configMouse:
		m.cfg.Mouse = !m.cfg.Mouse
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
			return nil
		}
		if m.cfg.Mouse {
			m.status = "the mouse is on, and the terminal will not select text with it"
			return tea.EnableMouseCellMotion
		}
		m.status = "the mouse is off, and text selection is the terminal's again"
		return tea.DisableMouse
	}

	return nil
}

func (m *Model) viewConfig() string {
	instruments, inputs := m.configShows()
	lines := []string{""}

	head := m.instrument().Name
	if m.first != firstRunDone {
		head = m.step()
	}

	if instruments {
		lines = append(lines, m.sectionHead("INSTRUMENT", head), "")
		m.clicks = append(m.clicks, clickable{top: headerLines + len(lines), count: len(song.Instruments)})
		for index, item := range song.Instruments {
			lines = append(lines, m.instrumentRow(item, index == m.configRow))
		}
		if m.first == firstRunInstrument {
			lines = append(lines,
				"",
				"  "+styleSubtle.Render("Whichever one is plugged in. The tuner is drawn from it while no"),
				"  "+styleSubtle.Render("song is open, and a search says which tabs are for it."),
				"",
				"  "+styleFaint.Render("A song brings its own tuning, so this is not asked about again."),
				"  "+styleFaint.Render("The input is the other question, and then the app opens."),
			)
		}
	}

	if inputs {
		if instruments {
			lines = append(lines, "")
		}
		right := plural(len(m.devices), "input")
		if m.first != firstRunDone {
			right = m.step()
		}
		lines = append(lines, m.sectionHead("AUDIO INPUT", right), "")

		tail := 3
		if m.first == firstRunInput {
			tail = 7
		} else {
			// under the inputs are the site list, the switch and the head of
			// the tuner, which keeps its meter only if the window has room
			tail += 8 + len(tabsite.Sites)
		}
		lines = append(lines, m.deviceRows(headerLines+len(lines), m.space()-len(lines)-tail)...)

		if m.first == firstRunInput {
			lines = append(lines,
				"",
				"  "+styleSubtle.Render("Which input it is plugged into. Nothing is listened to until this"),
				"  "+styleSubtle.Render("is answered, and it is only asked again if that input goes missing."),
				"",
				"  "+styleFaint.Render("Keeping it is the last step, and the app opens on the search."),
			)
		}
	}

	if m.first == firstRunDone {
		lines = append(lines, "", m.sectionHead("THE TAB SITE", m.cfg.Site), "")
		m.clicks = append(m.clicks, clickable{top: headerLines + len(lines),
			first: m.configCount() - len(tabsite.Sites) - 1, count: len(tabsite.Sites)})
		for index, item := range tabsite.Sites {
			lines = append(lines, m.siteRow(item, index))
		}

		lines = append(lines, "", m.sectionHead("THE MOUSE", ""), "")
		m.clicks = append(m.clicks, clickable{top: headerLines + len(lines),
			first: m.configCount() - 1, count: 1})
		lines = append(lines, m.mouseRow())

		// the tuner is the last thing on the screen and the cursor walks past
		// it: it is read and not answered, so it is a block and not a row
		lines = append(lines, "", m.sectionHead("THE TUNER", m.tunerHead()))
		lines = append(lines, m.tunerRows(m.space()-len(lines)-2)...)
	}

	lines = append(lines,
		"",
		"  "+styleFaint.Render("Songs are read from "+m.cfg.Library),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) instrumentRow(item song.Instrument, selected bool) string {
	paint := rowPaint(selected)

	mark, name := "   ", paint.of(styleInk).Render(item.Name)
	if selected {
		mark, name = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(item.Name)
	}

	note := paint.of(styleFaint).Render(fmt.Sprintf("%s · %s", plural(len(item.Tuning), "string"), item.Written()))
	if item.Name == m.cfg.Instrument {
		note = paint.of(styleOk).Render("playing") + paint.of(styleFaint).Render("   "+item.Written())
	}

	return paint.pad(mark+name, note+paint.of(styleFaint).Render(" "), m.width)
}

// deviceRows is the input list, scrolled under the cursor. A machine with a
// dozen inputs on it would otherwise draw past the bottom of the window.
func (m *Model) deviceRows(top, room int) []string {
	if len(m.devices) == 0 {
		return []string{"  " + styleFaint.Render("no input on the list, press r to read it again")}
	}
	if room < 1 {
		room = 1
	}

	start := 0
	if at := m.configRow - m.deviceOffset(); at >= room {
		start = at - room + 1
	}
	end := start + room
	if end > len(m.devices) {
		end = len(m.devices)
	}

	m.clicks = append(m.clicks, clickable{top: top, first: start + m.deviceOffset(), count: end - start})

	rows := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		rows = append(rows, m.deviceRow(m.devices[index], index+m.deviceOffset() == m.configRow))
	}

	return rows
}

func (m *Model) deviceRow(device deviceInfo, selected bool) string {
	paint := rowPaint(selected)

	mark, name := "   ", paint.of(styleInk).Render(device.Name)
	if selected {
		mark, name = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(device.Name)
	}

	note := paint.of(styleFaint).Render(fmt.Sprintf("%s · %d ch · %d Hz", device.Host, device.Channels, device.Rate))
	if m.chosen(device) {
		note = paint.of(styleOk).Render("in use") + paint.of(styleFaint).Render("   "+note)
	}

	return paint.pad(mark+name, note+paint.of(styleFaint).Render(" "), m.width)
}

// siteRow is one of the places a search reads from. The two do not answer
// alike, which is what the note beside a name is for: one has a dozen rated
// versions of a song and the other has one, in another language.
func (m *Model) siteRow(item tabsite.Info, index int) string {
	kind, at := m.configPick()
	selected := kind == configSite && at == index

	paint := rowPaint(selected)

	mark, name := "   ", paint.of(styleInk).Render(item.Name)
	if selected {
		mark, name = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(item.Name)
	}

	note := paint.of(styleFaint).Render(item.Note)
	if item.Name == m.cfg.Site {
		note = paint.of(styleOk).Render("reading") + paint.of(styleFaint).Render("   "+item.Note)
	}

	return paint.pad(mark+name, note+paint.of(styleFaint).Render(" "), m.width)
}

// mouseRow is the one switch on the screen. It says what it costs, because a
// terminal that is reporting the mouse is a terminal that no longer selects
// text with it.
func (m *Model) mouseRow() string {
	kind, _ := m.configPick()

	paint := rowPaint(kind == configMouse)

	name := "clicking and the wheel"
	mark, label := "   ", paint.of(styleInk).Render(name)
	if kind == configMouse {
		mark, label = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(name)
	}

	state := paint.of(styleFaint).Render("off") +
		paint.of(styleFaint).Render("   the terminal selects text as usual")
	if m.cfg.Mouse {
		state = paint.of(styleOk).Render("on") +
			paint.of(styleFaint).Render("   hold shift to select text")
	}

	return paint.pad(mark+label, state+paint.of(styleFaint).Render(" "), m.width)
}
