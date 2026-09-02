package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// Movement is vim everywhere: j down, k up, h back or left, l in or right, gg
// and G for the ends. The keys that do something particular to a screen are
// letters that say what they do, and all of them are on the screen, in the bar
// at the bottom and in full behind the question mark.
func (m *Model) key(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		return tea.Quit
	}

	m.fail = ""

	// the help is a mode of the whole app, and anything at all leaves it
	if m.helping {
		m.helping = false
		return nil
	}

	// a removal asks first, and anything that is not yes is no
	if m.removing {
		m.removing = false
		if msg.String() == "y" {
			return m.forget(m.found)
		}
		m.status = "left it where it was"
		return nil
	}

	if m.input.Focused() {
		return m.typing(msg)
	}

	key := msg.String()

	// esc is the way back to the normal mode, and it is answered here before
	// the screen under it sees a key that also means leave
	if key == "esc" && m.mode != modeNormal {
		m.normal()
		return nil
	}

	// gg is two keys, so the first is remembered and anything else forgets it
	if m.pendingG {
		m.pendingG = false
		if key == "g" {
			m.jump(-1)
			return nil
		}
	}

	// the first run is two steps and they are the whole of the app until they
	// are answered: there is nothing to walk to yet
	if m.first != firstRunDone {
		switch key {
		case "H", "L":
			return nil
		}
	}

	switch key {
	case "?":
		m.helping = true
		return nil
	case "q":
		return tea.Quit
	case "g":
		m.pendingG = true
		return nil
	case "G":
		m.jump(1)
		return nil
	// the screens are walked with these two and with nothing else. a number
	// key that jumps straight to one is a second scheme to remember, and the
	// buttons on the line above say which way each of the two goes
	case "L":
		return m.goTo(tabScreens[(m.tabHere()+1)%len(tabScreens)])
	case "H":
		return m.goTo(tabScreens[(m.tabHere()+len(tabScreens)-1)%len(tabScreens)])
	case "j", "down":
		m.move(1)
		return nil
	case "k", "up":
		m.move(-1)
		return nil
	}

	switch m.screen {
	case screenMusic:
		return m.keySearch(key)
	case screenSpotify:
		return m.keySpotify(key)
	case screenPractice:
		return m.keyPractice(key)
	case screenConfig:
		return m.keyConfig(key)
	}

	return nil
}

// move is j and k on whatever list the screen is showing. Every screen has one
// cursor at most, so the movement lives here instead of five times over.
func (m *Model) move(delta int) {
	switch m.screen {
	case screenMusic:
		m.found = clamp(m.found+delta, len(m.results))
	case screenSpotify:
		m.picked = clamp(m.picked+delta, m.spotifyRows())
	case screenConfig:
		m.configRow = clamp(m.configRow+delta, m.configCount())
	case screenPractice:
		if m.engine == nil {
			return
		}
		// up and down on a tab means the note before and the note after, since
		// there is nothing else to move through
		m.engine.Stop()
		m.engine.Seek(m.engine.Cursor() + delta)
	}
}

// jump is gg and G, the two ends of the same list.
func (m *Model) jump(direction int) {
	last := 0
	switch m.screen {
	case screenMusic:
		last = len(m.results) - 1
	case screenSpotify:
		last = m.spotifyRows() - 1
	case screenConfig:
		last = m.configCount() - 1
	case screenPractice:
		if m.engine == nil {
			return
		}
		m.engine.Stop()
		if direction < 0 {
			m.engine.Seek(0)
			return
		}
		m.engine.Seek(len(m.engine.Events) - 1)
		return
	}

	if last < 0 {
		last = 0
	}

	target := 0
	if direction > 0 {
		target = last
	}

	switch m.screen {
	case screenMusic:
		m.found = target
	case screenSpotify:
		m.picked = target
	case screenConfig:
		m.configRow = target
	}
}

func clamp(value, length int) int {
	if value < 0 || length == 0 {
		return 0
	}
	if value >= length {
		return length - 1
	}
	return value
}

func (m *Model) goTo(next screen) tea.Cmd {
	// leaving the practice screen stops the clock. coming back to a song that
	// kept counting while nobody was looking would report a wall of misses
	if m.screen == screenPractice && m.engine != nil {
		m.engine.Stop()
	}

	// the repeat mode is that screen's, and its keys mean something else on
	// every other one. what was picked is kept and is still looped over
	if next != screenPractice {
		m.setMode(modeNormal)
	}
	m.screen = next
	m.status = ""

	// the spotify library is read the first time that screen is opened
	if next == screenSpotify {
		return m.enterSpotify()
	}

	return nil
}

// ask opens the one text field of the app for a particular question.
func (m *Model) ask(what asking, placeholder, value string) tea.Cmd {
	m.setMode(modeInsert)
	m.asking = what
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	return textinputBlink()
}

func (m *Model) typing(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeInput()
		return nil

	case tea.KeyEnter:
		value := strings.TrimSpace(m.input.Value())
		asked := m.asking
		m.closeInput()

		if value == "" {
			return nil
		}

		switch asked {
		case askingQuery:
			return m.askUltimate(value)
		}
		return nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) closeInput() {
	m.input.Blur()
	m.input.SetValue("")
	m.asking = askingNothing
	m.setMode(modeNormal)
}

// open loads a song into the practice screen. It is the only way in now: every
// song comes off a row of the search.
func (m *Model) open(loaded *song.Song) {
	m.current = loaded
	m.engine = practice.New(loaded, practice.Wait)
	m.engine.Speed = m.cfg.Speed
	m.tab = song.NewTab(loaded, m.engine.Events)
	m.screen = screenPractice
	m.status = ""
}

func (m *Model) keyPractice(key string) tea.Cmd {
	if m.engine == nil {
		return nil
	}

	switch key {
	case "v":
		m.highway = !m.highway

	case " ":
		// in the repeat mode the space is what picks a passage, since the
		// measure under the cursor is the thing already being looked at
		if m.mode == modeRepeat {
			m.pickRepeat()
			return nil
		}

		// wait mode has no clock to start, and saying stopped there would be
		// answering a question nobody asked
		if m.engine.Mode == practice.Wait {
			m.status = "wait mode has no clock, just play the note"
			return nil
		}
		if m.engine.Running() {
			m.engine.Stop()
			m.status = "stopped"
			return nil
		}
		m.engine.Start(time.Now())
		m.status = ""

	case "m":
		if m.engine.Mode == practice.Wait {
			// a text tab carries the notes and their order and nothing else.
			// running a clock over it would mark somebody wrong for playing
			// the rhythm the song actually has
			if m.current.Untimed {
				m.fail = "this tab came as text, so it has no rhythm to run a clock against"
				return nil
			}
			m.engine.Mode = practice.Tempo
		} else {
			m.engine.Mode = practice.Wait
		}
		m.engine.Stop()
		m.engine.Reset()

	case "r":
		// r a second time is how the passage is dropped: the hand is on the
		// key already and the mode it drops is the one it turned on
		if m.mode == modeRepeat {
			m.engine.ClearRepeat()
			m.normal()
			m.status = "the passage was let go"
			return nil
		}
		m.setMode(modeRepeat)
		m.status = "space marks the measure under the cursor, r lets it go"

	case "R":
		m.engine.Reset()
		m.status = ""

	case "h", "left":
		m.engine.Stop()
		m.engine.Seek(m.engine.Cursor() - 1)
	case "l", "right":
		m.engine.Stop()
		m.engine.Seek(m.engine.Cursor() + 1)

	case "[":
		m.engine.Stop()
		m.seekMeasure(-1)
	case "]":
		m.engine.Stop()
		m.seekMeasure(1)

	case "+", "=":
		m.setSpeed(m.engine.Speed + 0.05)
	case "-", "_":
		m.setSpeed(m.engine.Speed - 0.05)

	// backspace is the way out of a song, the same as esc: the screen it goes
	// back to is the list the song was opened from
	case "esc", "backspace":
		return m.goTo(screenMusic)
	}

	return nil
}

// seekMeasure jumps a whole measure, which is how a passage gets repeated
// without playing everything before it every time.
func (m *Model) seekMeasure(step int) {
	events := m.engine.Events
	if len(events) == 0 {
		return
	}

	cursor := m.engine.Cursor()
	if cursor >= len(events) {
		cursor = len(events) - 1
	}

	target := events[cursor].Measure + step
	if step < 0 && cursor > 0 && events[cursor-1].Measure == events[cursor].Measure {
		// already inside a measure, so back goes to its first note first
		target = events[cursor].Measure
	}
	if target < 1 {
		target = 1
	}

	m.engine.SeekMeasure(target)
}

func (m *Model) setSpeed(speed float64) {
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 1.5 {
		speed = 1.5
	}

	m.engine.Speed = speed
	m.cfg.Speed = speed
	m.status = fmt.Sprintf("speed %.0f%%", speed*100)
	_ = m.cfg.Save()
}

func (m *Model) keyConfig(key string) tea.Cmd {
	switch key {
	case "l", "enter":
		return m.keepConfig()

	case "h":
		m.back()
		return nil

	case "esc":
		if m.first != firstRunDone {
			m.later()
		}
		return nil

	case "r":
		// a device plugged in after the app started is on no list read before it
		m.status = "reading the input list again"
		return m.askDevices()
	}

	return nil
}
