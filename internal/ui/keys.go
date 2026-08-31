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

	if m.input.Focused() {
		return m.typing(msg)
	}

	key := msg.String()

	// gg is two keys, so the first is remembered and anything else forgets it
	if m.pendingG {
		m.pendingG = false
		if key == "g" {
			m.jump(-1)
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
	case "L", "tab":
		m.goTo(screen((int(m.screen) + 1) % len(screenNames)))
		return nil
	case "H", "shift+tab":
		m.goTo(screen((int(m.screen) + len(screenNames) - 1) % len(screenNames)))
		return nil
	case "1", "2", "3", "4", "5", "6":
		m.goTo(screen(int(key[0] - '1')))
		return nil
	case "j", "down":
		m.move(1)
		return nil
	case "k", "up":
		m.move(-1)
		return nil
	}

	switch m.screen {
	case screenLibrary:
		return m.keyLibrary(key)
	case screenSearch:
		return m.keySearch(key)
	case screenPractice:
		return m.keyPractice(key)
	case screenAnalyze:
		return m.keyAnalyze(key)
	case screenSetup:
		return m.keySetup(key)
	}

	return nil
}

// move is j and k on whatever list the screen is showing. Every screen has one
// cursor at most, so the movement lives here instead of five times over.
func (m *Model) move(delta int) {
	switch m.screen {
	case screenLibrary:
		if m.tracks != nil {
			m.track = clamp(m.track+delta, len(m.tracks))
			return
		}
		m.pick = clamp(m.pick+delta, len(m.filtered()))
	case screenSearch:
		m.found = clamp(m.found+delta, len(m.results))
	case screenAnalyze:
		m.reportRow = clamp(m.reportRow+delta, m.problemCount())
	case screenSetup:
		m.device = clamp(m.device+delta, len(m.devices))
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
	case screenLibrary:
		if m.tracks != nil {
			last = len(m.tracks) - 1
		} else {
			last = len(m.filtered()) - 1
		}
	case screenSearch:
		last = len(m.results) - 1
	case screenAnalyze:
		last = m.problemCount() - 1
	case screenSetup:
		last = len(m.devices) - 1
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
	case screenLibrary:
		if m.tracks != nil {
			m.track = target
			return
		}
		m.pick = target
	case screenSearch:
		m.found = target
	case screenAnalyze:
		m.reportRow = target
	case screenSetup:
		m.device = target
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

func (m *Model) goTo(next screen) {
	// leaving the practice screen stops the clock. coming back to a song that
	// kept counting while nobody was looking would report a wall of misses
	if m.screen == screenPractice && m.engine != nil {
		m.engine.Stop()
	}
	m.screen = next
	m.status = ""
}

// ask opens the one text field of the app for a particular question.
func (m *Model) ask(what asking, placeholder, value string) tea.Cmd {
	m.asking = what
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	return textinputBlink()
}

func (m *Model) typing(msg tea.KeyMsg) tea.Cmd {
	// the filter reads as you type, or it would be a dialog and not a filter
	if m.asking == askingFilter {
		switch msg.Type {
		case tea.KeyEsc:
			m.filter = ""
			m.closeInput()
			return nil
		case tea.KeyEnter:
			m.closeInput()
			return nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.filter = m.input.Value()
		m.pick = clamp(m.pick, len(m.filtered()))
		return cmd
	}

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
			m.query = value
			m.results = nil
			m.found = 0
			m.seeking = true
			m.status = "searching songsterr for " + value
			return m.searchSongsterr(value)

		case askingImport, askingRecording:
			path := expand(value)
			if _, err := os.Stat(path); err != nil {
				m.fail = err.Error()
				return nil
			}
			if asked == askingImport {
				m.pending = path
				m.status = "reading " + filepath.Base(path)
				return m.run("gpimport.py", path)
			}
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

func (m *Model) closeInput() {
	m.input.Blur()
	m.input.SetValue("")
	m.asking = askingNothing
}

func (m *Model) keyLibrary(key string) tea.Cmd {
	if m.tracks != nil {
		switch key {
		case "l", "enter":
			return m.importTrack()
		case "h", "esc":
			m.tracks = nil
		}
		return nil
	}

	switch key {
	case "l", "enter":
		return m.openSong()
	case "/":
		return m.ask(askingFilter, "filter the library", m.filter)
	case "h", "esc":
		if m.filter != "" {
			m.filter = ""
			m.pick = 0
		}
	case "i":
		return m.ask(askingImport, "path to a .gp3, .gp4, .gp5 or .gpx file", "")
	case "r":
		return m.loadSongs()
	case "d":
		return m.deleteSong()
	}

	return nil
}

func (m *Model) importTrack() tea.Cmd {
	if m.track >= len(m.tracks) {
		return nil
	}

	chosen := m.tracks[m.track]
	if !chosen.Playable {
		m.fail = "that track has no strings, so there is no tab to draw"
		return nil
	}

	out := filepath.Join(m.cfg.Library, slug(filepath.Base(m.pending), chosen.Name)+".json")
	m.status = "importing " + chosen.Name

	return m.run("gpimport.py", m.pending, "--track", fmt.Sprint(chosen.Index), "--out", out)
}

// filtered is the library narrowed by whatever was typed after the slash. The
// match is over the title, the artist and the track, since all three are on the
// row and any of them is a reasonable thing to type.
func (m *Model) filtered() []*song.Song {
	if m.filter == "" {
		return m.songs
	}

	needle := strings.ToLower(m.filter)
	var kept []*song.Song
	for _, item := range m.songs {
		haystack := strings.ToLower(item.Title + " " + item.Artist + " " + item.Track)
		if strings.Contains(haystack, needle) {
			kept = append(kept, item)
		}
	}

	return kept
}

func (m *Model) openSong() tea.Cmd {
	list := m.filtered()
	if m.pick >= len(list) {
		return nil
	}

	m.current = list[m.pick]
	m.engine = practice.New(m.current, practice.Wait)
	m.engine.Speed = m.cfg.Speed
	m.tab = song.NewTab(m.current, m.engine.Events)
	m.screen = screenPractice
	m.status = ""

	return nil
}

func (m *Model) deleteSong() tea.Cmd {
	list := m.filtered()
	if m.pick >= len(list) {
		return nil
	}

	path := list[m.pick].Path
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

func (m *Model) keyPractice(key string) tea.Cmd {
	if m.engine == nil {
		return nil
	}

	switch key {
	case "v":
		m.highway = !m.highway

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

func (m *Model) keyAnalyze(key string) tea.Cmd {
	if key == "a" {
		if m.current == nil {
			m.fail = "pick a song on the library screen first"
			return nil
		}
		return m.ask(askingRecording, "path to a recording of you playing it", "")
	}
	return nil
}

func (m *Model) keySetup(key string) tea.Cmd {
	switch key {
	case "l", "enter":
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

	case "s":
		m.status = "opening the spotify login in your browser"
		return m.spotify("login")
	}

	return nil
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
