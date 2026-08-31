package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// key routes a keystroke. The text field takes the whole keyboard while it is
// focused, apart from the two keys that close it, or typing a path would
// switch screens on every letter that happens to be a digit.
func (m *Model) key(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		return tea.Quit
	}

	m.fail = ""

	if m.input.Focused() {
		return m.typing(msg)
	}

	switch msg.String() {
	case "q":
		return tea.Quit
	case "tab":
		m.goTo(screen((int(m.screen) + 1) % len(screenNames)))
		return nil
	case "shift+tab":
		m.goTo(screen((int(m.screen) + len(screenNames) - 1) % len(screenNames)))
		return nil
	case "1", "2", "3", "4", "5":
		m.goTo(screen(int(msg.String()[0] - '1')))
		return nil
	}

	switch m.screen {
	case screenLibrary:
		return m.keyLibrary(msg)
	case screenPractice:
		return m.keyPractice(msg)
	case screenAnalyze:
		return m.keyAnalyze(msg)
	case screenSetup:
		return m.keySetup(msg)
	}

	return nil
}

func (m *Model) goTo(next screen) {
	// leaving the practice screen stops the clock. coming back to a song that
	// kept counting while nobody was looking would report a wall of misses
	if m.screen == screenPractice && m.engine != nil {
		m.engine.Stop()
	}
	m.screen = next
	m.status = ""
}

func (m *Model) typing(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.input.Blur()
		m.input.SetValue("")
		m.asking = askingNothing
		return nil

	case tea.KeyEnter:
		path := expand(m.input.Value())
		asked := m.asking
		m.input.Blur()
		m.input.SetValue("")
		m.asking = askingNothing

		if path == "" {
			return nil
		}
		if _, err := os.Stat(path); err != nil {
			m.fail = err.Error()
			return nil
		}

		switch asked {
		case askingImport:
			m.pending = path
			m.status = "reading " + filepath.Base(path)
			return m.run("gpimport.py", path)
		case askingRecording:
			if m.current == nil {
				m.fail = "pick a song on the library screen first"
				return nil
			}
			m.report = nil
			m.progress = 0
			m.running = true
			m.status = "listening to " + filepath.Base(path)
			return m.run("analyze.py", "--song", m.current.Path, "--audio", path)
		}
		return nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) keyLibrary(msg tea.KeyMsg) tea.Cmd {
	// the track list of a guitar pro file is a question, and while it is open
	// it owns the arrows
	if m.tracks != nil {
		return m.keyTracks(msg)
	}

	switch msg.String() {
	case "up", "k":
		if m.pick > 0 {
			m.pick--
		}
	case "down", "j":
		if m.pick < len(m.songs)-1 {
			m.pick++
		}
	case "enter":
		return m.openSong()
	case "i":
		m.asking = askingImport
		m.input.Placeholder = "path to a .gp3, .gp4, .gp5 or .gpx file"
		m.input.Focus()
		return textinputBlink()
	case "r":
		return m.loadSongs()
	case "d":
		return m.deleteSong()
	}

	return nil
}

func (m *Model) keyTracks(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if m.track > 0 {
			m.track--
		}
	case "down", "j":
		if m.track < len(m.tracks)-1 {
			m.track++
		}
	case "esc":
		m.tracks = nil
	case "enter":
		chosen := m.tracks[m.track]
		if !chosen.Playable {
			m.fail = "that track has no strings, so there is no tab to draw"
			return nil
		}
		out := filepath.Join(m.cfg.Library, slug(filepath.Base(m.pending), chosen.Name)+".json")
		m.status = "importing " + chosen.Name
		return m.run("gpimport.py", m.pending, "--track", fmt.Sprint(chosen.Index), "--out", out)
	}

	return nil
}

func (m *Model) openSong() tea.Cmd {
	if m.pick >= len(m.songs) {
		return nil
	}

	m.current = m.songs[m.pick]
	m.engine = practice.New(m.current, practice.Wait)
	m.engine.Speed = m.cfg.Speed
	m.tab = song.NewTab(m.current, m.engine.Events)
	m.screen = screenPractice
	m.status = ""

	return nil
}

func (m *Model) deleteSong() tea.Cmd {
	if m.pick >= len(m.songs) {
		return nil
	}

	path := m.songs[m.pick].Path
	if err := os.Remove(path); err != nil {
		m.fail = err.Error()
		return nil
	}
	if m.current != nil && m.current.Path == path {
		m.current, m.engine, m.tab = nil, nil, nil
	}

	m.status = "removed " + filepath.Base(path)
	return m.loadSongs()
}

func (m *Model) keyPractice(msg tea.KeyMsg) tea.Cmd {
	if m.engine == nil {
		return nil
	}

	switch msg.String() {
	case " ":
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
			m.engine.Mode = practice.Tempo
		} else {
			m.engine.Mode = practice.Wait
		}
		m.engine.Stop()
		m.engine.Reset()

	case "r":
		m.engine.Reset()
		m.status = ""

	case "left", "h":
		m.engine.Stop()
		m.engine.Seek(m.engine.Cursor() - 1)
	case "right", "l":
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

	case "esc":
		m.goTo(screenLibrary)
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

func (m *Model) keyAnalyze(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "a":
		if m.current == nil {
			m.fail = "pick a song on the library screen first"
			return nil
		}
		m.asking = askingRecording
		m.input.Placeholder = "path to a recording of you playing it"
		m.input.Focus()
		return textinputBlink()

	case "up", "k":
		if m.reportRow > 0 {
			m.reportRow--
		}
	case "down", "j":
		if m.report != nil && m.reportRow < len(m.report.Notes)-1 {
			m.reportRow++
		}
	}

	return nil
}

func (m *Model) keySetup(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if m.device > 0 {
			m.device--
		}
	case "down", "j":
		if m.device < len(m.devices)-1 {
			m.device++
		}
	case "enter":
		if m.device >= len(m.devices) {
			return nil
		}
		chosen := m.devices[m.device]
		m.cfg.Device = chosen.Index
		m.cfg.Rate = chosen.Rate
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
			return nil
		}
		m.status = "input is " + chosen.Name
		return m.listen()
	}

	return nil
}

// keyHints is the line at the bottom. It says what this screen does, not what
// every screen does, since the tab key is on all of them and saying so five
// times helps nobody.
func (m *Model) keyHints() string {
	if m.input.Focused() {
		return "enter  confirm    esc  cancel"
	}

	switch m.screen {
	case screenLibrary:
		if m.tracks != nil {
			return "↑↓  track    enter  import    esc  cancel"
		}
		return "↑↓  song    enter  practice    i  import guitar pro    d  remove    r  reload"
	case screenPractice:
		return "space  run    m  mode    ←→  note    []  measure    +-  speed    r  restart    esc  library"
	case screenTuner:
		return "play an open string"
	case screenAnalyze:
		return "a  pick a recording    ↑↓  scroll"
	case screenSetup:
		return "↑↓  device    enter  use it"
	}

	return ""
}

// slug names the imported file after the guitar pro file and the track, so two
// tracks of the same song do not overwrite each other.
func slug(file, track string) string {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	name := strings.ToLower(base + "-" + track)

	var out strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}

	return strings.Trim(collapse(out.String()), "-")
}

func collapse(text string) string {
	for strings.Contains(text, "--") {
		text = strings.ReplaceAll(text, "--", "-")
	}
	return text
}
