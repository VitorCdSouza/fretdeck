package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/cifraclub"
	"github.com/VitorCdSouza/fretdeck/internal/config"
	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
	"github.com/VitorCdSouza/fretdeck/internal/ultimate"
)

// TestMain points the config folder at a temporary one. Saving is what a
// screen does when a device or a speed is picked, and a run of go test used to
// write the model of the test over the config of whoever ran it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fretdeck")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

// riff is a song with two measures, a chord and a two digit fret, which is
// enough for every rule the practice screen draws by.
func riff() *song.Song {
	return &song.Song{
		Title:  "Test Riff",
		Artist: "Nobody",
		Track:  "Guitar 1",
		Tempo:  100,
		Tuning: []song.String{{Number: 1, Midi: 64}, {Number: 2, Midi: 59}, {Number: 3, Midi: 55},
			{Number: 4, Midi: 50}, {Number: 5, Midi: 45}, {Number: 6, Midi: 40}},
		Measures: []song.Measure{
			{Index: 1, Beat: 0, Time: 0, Tempo: 100, Signature: [2]int{4, 4}},
			{Index: 2, Beat: 4, Time: 2.4, Tempo: 100, Signature: [2]int{4, 4}},
		},
		Notes: []song.Note{
			{Measure: 1, Beat: 0, Time: 0, Dur: 0.3, String: 6, Fret: 3, Midi: 43},
			{Measure: 1, Beat: 1, Time: 0.6, Dur: 0.3, String: 5, Fret: 12, Midi: 57},
			{Measure: 1, Beat: 2, Time: 1.2, Dur: 0.3, String: 4, Fret: 5, Midi: 55},
			{Measure: 2, Beat: 4, Time: 2.4, Dur: 0.6, String: 3, Fret: 0, Midi: 55},
			{Measure: 2, Beat: 4, Time: 2.4, Dur: 0.6, String: 2, Fret: 1, Midi: 60},
		},
	}
}

func model(t *testing.T) *Model {
	t.Helper()

	input := textinput.New()
	input.Prompt = "  "

	loaded := riff()
	engine := practice.New(loaded)

	return &Model{
		cfg:     config.Config{Device: 1, Rate: 44100, Library: "/home/somebody/fretdeck/songs"},
		width:   96,
		height:  30,
		songs:   []*song.Song{loaded},
		current: loaded,
		engine:  engine,
		tab:     song.NewTab(loaded, engine.Events),
		input:   input,
		devices: []deviceInfo{
			{Index: 1, Name: "HDA Intel PCH: ALC897 Analog", Host: "ALSA", Channels: 2, Rate: 44100},
			{Index: 11, Name: "default", Host: "ALSA", Channels: 128, Rate: 44100, Default: true},
		},
	}
}

// TestEveryScreenFitsItsWindow is the guard against a view that scrolls the
// terminal, which in the alt screen means the header walks off the top.
func TestEveryScreenFitsItsWindow(t *testing.T) {
	m := model(t)
	m.engine.Heard(44)

	for index := range screenNames {
		which := screen(index)
		m.screen = which
		lines := strings.Split(m.View(), "\n")
		if len(lines) > m.height {
			t.Fatalf("%s draws %d lines into a window of %d", screenNames[which], len(lines), m.height)
		}
		for index, line := range lines {
			if width := lipgloss.Width(line); width > m.width {
				t.Fatalf("%s line %d is %d cells wide, the window is %d", screenNames[which], index, width, m.width)
			}
		}
	}
}

// TestTheHelpFitsTheWindowToo covers the view that is not a screen of its own
// and so is missed by the loop above.
func TestTheHelpFitsTheWindowToo(t *testing.T) {
	m := model(t)

	m.helping = true
	mustFit(t, m, "help")

	// the two faces of the first run that the loop over the screens never sees
	m.helping = false
	m.screen = screenConfig
	for _, step := range []firstRun{firstRunInstrument, firstRunInput} {
		m.first = step
		m.startingRow()
		mustFit(t, m, "the first run")
	}
	m.first = firstRunDone
}

func mustFit(t *testing.T, m *Model, what string) {
	t.Helper()

	lines := strings.Split(m.View(), "\n")
	if len(lines) > m.height {
		t.Fatalf("%s draws %d lines into a window of %d", what, len(lines), m.height)
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("%s line %d is %d cells wide, the window is %d", what, index, width, m.width)
		}
	}
}

// TestMovementIsVimOnEveryList is the promise the help makes.
func TestMovementIsVimOnEveryList(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = []finding{{Title: "one"}, {Title: "two"}, {Title: "three"}}

	m.move(1)
	if m.found != 1 {
		t.Fatalf("j did not move down, cursor is %d", m.found)
	}

	m.jump(1)
	if m.found != 2 {
		t.Fatalf("G did not land on the last row, cursor is %d", m.found)
	}

	m.jump(-1)
	if m.found != 0 {
		t.Fatalf("gg did not land on the first row, cursor is %d", m.found)
	}

	// past the end stays on the end rather than wrapping, which is what a
	// held down key would otherwise do
	m.found = 2
	m.move(1)
	if m.found != 2 {
		t.Fatalf("j past the end moved to %d", m.found)
	}
}

// TestMovingOnAnEmptyListIsNotACrash is the state the app opens in.
func TestMovingOnAnEmptyListIsNotACrash(t *testing.T) {
	m := model(t)
	m.songs = nil
	m.results = nil
	m.screen = screenMusic
	m.focus = paneSearch

	m.move(1)
	m.jump(1)

	if m.musicCursor() != fieldRow {
		t.Fatalf("want the cursor on the field, got %d", m.musicCursor())
	}
}

// TestTheMarkerSitsOverTheCursor is the alignment that makes the practice
// screen readable, and the one that breaks whenever the tab indent changes.
func TestTheMarkerSitsOverTheCursor(t *testing.T) {
	m := model(t)
	m.engine.Seek(1)
	m.screen = screenPractice

	lines := strings.Split(m.View(), "\n")
	var caret, tab string
	for index, line := range lines {
		if strings.Contains(line, marker) {
			caret = line
			// the header of the tab, then the six strings from the top down
			tab = lines[index+6]
			break
		}
	}

	if caret == "" {
		t.Fatal("the practice screen drew no marker")
	}

	at := strings.Index(caret, marker)
	runes := []rune(stripAnsi(tab))
	if at >= len(runes) {
		t.Fatalf("the marker at %d is past the end of the tab line", at)
	}
	if runes[at] != '1' {
		t.Fatalf("the marker points at %q, the twelfth fret starts with 1", string(runes[at]))
	}
}

// stripAnsi is only for the tests: the views are styled and the columns can
// only be counted on the text under the escapes.
func stripAnsi(text string) string {
	var out strings.Builder
	inside := false
	for _, r := range text {
		switch {
		case r == 0x1b:
			inside = true
		case inside && r == 'm':
			inside = false
		case !inside:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// TestTheFirstRunAsksTheInstrumentThenTheInput is the whole of it: two
// questions in that order, each kept, and neither asked again after that.
func TestTheFirstRunAsksTheInstrumentThenTheInput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New()
	if m.first != firstRunInstrument || m.screen != screenConfig {
		t.Fatalf("a first run opens on the instrument question, got %v on %s", m.first, screenNames[m.screen])
	}

	m.devices = []deviceInfo{
		{Index: 1, Name: "line in", Rate: 44100},
		{Index: 11, Name: "default", Rate: 48000, Default: true},
	}

	// the third row is the bass, and keeping it moves on to the input
	m.configRow = 2
	m.keepConfig()

	if m.cfg.Instrument != "bass" {
		t.Fatalf("the instrument was not kept, config says %q", m.cfg.Instrument)
	}
	if m.first != firstRunInput {
		t.Fatalf("want the input question next, got %v", m.first)
	}
	if m.configCount() != len(m.devices) {
		t.Fatalf("the input question shows %d rows, want the %d inputs", m.configCount(), len(m.devices))
	}

	m.configRow = 0
	m.keepConfig()

	if m.cfg.Device != 1 || m.cfg.Rate != 44100 {
		t.Fatalf("the input was not kept, config says device %d at %d Hz", m.cfg.Device, m.cfg.Rate)
	}
	if m.first != firstRunDone || m.screen == screenConfig {
		t.Fatal("both answers are in, so the questions are done with and off the screen")
	}

	// and the next run asks neither, which is the point of keeping them
	again := New()
	if again.first != firstRunDone || again.screen == screenConfig {
		t.Fatalf("a second run asked again, first is %v", again.first)
	}
	if again.cfg.Instrument != "bass" || again.cfg.Device != 1 {
		t.Fatalf("the answers did not survive the run, got %q on device %d", again.cfg.Instrument, again.cfg.Device)
	}
}

// TestAnInputThatWentMissingIsSaidOutLoud is the trap this replaced: falling
// back to the system default without a word is how somebody ends up
// practising into the laptop microphone.
func TestAnInputThatWentMissingIsSaidOutLoud(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	gone := `{"devices":[{"index":3,"name":"laptop microphone","host":"ALSA","channels":1,"rate":44100,"default":true}]}`
	m.handle(bridge.Event{Event: bridge.EventDevices, Data: json.RawMessage(gone)})

	if m.cfg.Device != 1 {
		t.Fatalf("the saved input was overwritten with %d", m.cfg.Device)
	}
	if m.screen != screenConfig {
		t.Fatalf("want the config screen to ask again, got %s", screenNames[m.screen])
	}
	if m.fail == "" {
		t.Fatal("the input went missing and nothing was said about it")
	}
}

// TestAnInputThatIsStillThereIsNotWorthMentioning is the other half: the list
// coming back with the saved index on it opens the stream and says nothing.
func TestAnInputThatIsStillThereIsNotWorthMentioning(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	here := `{"devices":[{"index":1,"name":"line in","host":"ALSA","channels":2,"rate":44100}]}`
	m.handle(bridge.Event{Event: bridge.EventDevices, Data: json.RawMessage(here)})

	if m.screen != screenPractice || m.fail != "" {
		t.Fatalf("nothing was wrong, got %s saying %q", screenNames[m.screen], m.fail)
	}
}

// TestAnInputKeptByNameSurvivesTheRenumbering is why the choice is kept by
// name: the sound server hands out an index that anything plugged in moves,
// and the row at the old index is another device entirely.
func TestAnInputKeptByNameSurvivesTheRenumbering(t *testing.T) {
	m := model(t)
	m.screen = screenPractice
	m.cfg.Source, m.cfg.Device = "alsa_input.interface", 1

	moved := `{"devices":[` +
		`{"id":"alsa_input.webcam","index":9,"name":"webcam","host":"PipeWire","channels":1,"rate":48000,"default":true},` +
		`{"id":"alsa_input.interface","index":9,"name":"line in","host":"PipeWire","channels":2,"rate":48000}]}`
	m.handle(bridge.Event{Event: bridge.EventDevices, Data: json.RawMessage(moved)})

	if m.screen != screenPractice || m.fail != "" {
		t.Fatalf("the input was on the list, got %s saying %q", screenNames[m.screen], m.fail)
	}
	if m.deviceName() != "line in" {
		t.Fatalf("want the input that was kept, got %q", m.deviceName())
	}
}

// TestAnInputKeptByNameIsNotAnotherOneAtThatIndex is the other half: a list
// without the name on it asks again rather than taking whatever is there.
func TestAnInputKeptByNameIsNotAnotherOneAtThatIndex(t *testing.T) {
	m := model(t)
	m.screen = screenPractice
	m.cfg.Source, m.cfg.Device = "alsa_input.interface", 1

	other := `{"devices":[{"id":"alsa_input.webcam","index":1,"name":"webcam","host":"PipeWire","channels":1,"rate":48000}]}`
	m.handle(bridge.Event{Event: bridge.EventDevices, Data: json.RawMessage(other)})

	if m.screen != screenConfig || m.fail == "" {
		t.Fatalf("want the config screen to ask again, got %s saying %q", screenNames[m.screen], m.fail)
	}
	if m.cfg.Source != "alsa_input.interface" {
		t.Fatalf("the saved input was overwritten with %q", m.cfg.Source)
	}
}

// TestTheTunerOpensOnTheInstrumentWithNoSong is what the instrument answer is
// for: a tuner with nothing loaded still knows how many strings there are.
func TestTheTunerOpensOnTheInstrumentWithNoSong(t *testing.T) {
	m := model(t)
	m.current, m.engine, m.tab = nil, nil, nil
	m.cfg.Instrument = "bass"
	m.screen = screenConfig

	// the meter is the last block on the screen and a short window keeps only
	// its head, so the strings are asked of a window with room for them
	m.height = 40

	if got := len(m.instrument().Tuning); got != 4 {
		t.Fatalf("a bass has four strings, the tuner was handed %d", got)
	}

	// the low E of a bass is an octave under the low E of a guitar
	drawn := stripAnsi(m.View())
	if !strings.Contains(drawn, "E1") || strings.Contains(drawn, "E2") {
		t.Fatal("the tuner drew guitar strings with a bass plugged in")
	}
}

// TestABassPlayerSeesTheBassTabsFirst is the other half of the instrument: a
// search answers for both instruments in one list.
func TestABassPlayerSeesTheBassTabsFirst(t *testing.T) {
	m := model(t)
	m.cfg.Instrument = "bass"
	m.results = []finding{
		{Title: "one", Kind: tabsite.KindChord},
		{Title: "two", Kind: tabsite.KindTab},
		{Title: "three", Kind: tabsite.KindBass},
	}

	m.sortByInstrument()

	if m.results[0].Title != "three" {
		t.Fatalf("the bass tab has to come first, got %q", m.results[0].Title)
	}

	m.cfg.Instrument = "guitar"
	m.sortByInstrument()

	if m.results[0].Kind != tabsite.KindTab {
		t.Fatalf("a guitar player sees the guitar tab first, got %q", m.results[0].Kind)
	}
}

// TestTheAppOpensOnWhatWasPlayed is the one way in: the search screen with
// nothing typed is the way back to whatever was being worked on.
func TestTheAppOpensOnWhatWasPlayed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	kept := config.Recent{}
	kept.Remember(config.Entry{Artist: "Nobody", Title: "Test Riff", File: "/gone/test-riff.json"})
	if err := kept.Save(); err != nil {
		t.Fatal(err)
	}

	m := New()
	if m.focus != paneRecent {
		t.Fatal("the keys open on what was played, not on an empty search")
	}
	if len(m.kept) != 1 || m.kept[0].Title != "Test Riff" {
		t.Fatalf("the recent list did not come back, got %d rows", len(m.kept))
	}

	// the file is not on disk, and the entry stays anyway: the page it came
	// from is still the answer to finding it again
	if m.kept[0].Have() {
		t.Fatal("a song that is not on disk is not in the library")
	}
}

// TestRemovingADownloadedSongAsksFirst is the difference between d on a file
// and d on a line: one of them is the only copy of a tab that was read in.
func TestRemovingADownloadedSongAsksFirst(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	path := filepath.Join(dir, "test-riff.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := model(t)
	m.songs[0].Path = path
	m.recent.Remember(config.Entry{Artist: "Nobody", Title: "Test Riff", File: path})
	m.screen = screenMusic
	m.showRecent()

	if !m.kept[0].Have() {
		t.Fatal("the song is on disk, so the row has to say so")
	}

	m.keySearch("d")
	if !m.removing {
		t.Fatal("d on a downloaded song has to ask before it does anything")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the file went before the question was answered")
	}

	// anything that is not yes is no
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.removing {
		t.Fatal("the question is answered by any key")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("no removed the file anyway")
	}

	m.keySearch("d")
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("yes left the file where it was")
	}
	if len(m.recent.Entries) != 0 {
		t.Fatalf("the file and the entry go together, %d entries left", len(m.recent.Entries))
	}
}

// TestForgettingASongThatIsNotHereDoesNotAsk is the other half of d: there is
// nothing to lose in a line that only says a song was looked at.
func TestForgettingASongThatIsNotHereDoesNotAsk(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.songs = nil
	m.recent.Remember(config.Entry{Artist: "Nobody", Title: "Only Looked At",
		URL: "https://tabs.ultimate-guitar.com/tab/nobody/only-looked-at-1"})
	m.screen = screenMusic
	m.showRecent()

	m.keySearch("d")

	if m.removing {
		t.Fatal("there is no file to lose, so there is nothing to ask about")
	}
	if len(m.recent.Entries) != 0 {
		t.Fatalf("the entry is still on the list, %d left", len(m.recent.Entries))
	}
}

// TestOneLookupAnswersEveryVersionOfASong is what keeps twenty rows down to
// one request: they are all the one song and songsterr has the one answer.
func TestOneLookupAnswersEveryVersionOfASong(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.showTabs([]tabsite.Result{
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 1, URL: "one"},
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 2, URL: "two"},
		{Artist: "Nobody", Title: "Another One", Kind: tabsite.KindTab, Version: 1, URL: "three"},
	})

	if stillLooking(m.results) != 2 {
		t.Fatalf("both songs wait on an answer, %d do", stillLooking(m.results))
	}

	keys := map[string]bool{}
	for _, item := range m.results {
		keys[item.Key] = true
	}
	if len(keys) != 2 {
		t.Fatalf("three transcriptions of two songs is two lookups, got %d", len(keys))
	}

	m.answered(lookupMsg{key: m.results[0].Key, level: 4, found: true})

	// the versions are opened after the answer landed, and they take it from
	// the row they came out of rather than asking again
	m.found = 0
	m.expand(0)

	for _, item := range m.results {
		if item.Title != "Test Riff" {
			continue
		}
		if item.State != lookupDone || item.Level != 4 {
			t.Fatalf("version %d did not take the answer, state %d level %d",
				item.Version, item.State, item.Level)
		}
	}
	if stillLooking(m.results) != 1 {
		t.Fatalf("the other song is still waiting, %d rows are", stillLooking(m.results))
	}
}

// TestSearchGroupsTheVersionsOfASong is the list a search draws: one row per
// song however many transcriptions of it came back, and the best liked of them
// is the row.
func TestSearchGroupsTheVersionsOfASong(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.showTabs([]tabsite.Result{
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 1, Rating: 5, Votes: 2, URL: "one"},
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 2, Rating: 4.7, Votes: 300, URL: "two"},
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindBass, Version: 1, Rating: 4.5, Votes: 40, URL: "bass"},
		{Artist: "Nobody", Title: "Another One", Kind: tabsite.KindTab, Version: 1, Rating: 4, Votes: 10, URL: "four"},
	})

	if len(m.results) != 3 {
		t.Fatalf("two songs and a bass part of one is three rows, got %d", len(m.results))
	}

	head := m.results[0]
	if head.Count != 2 {
		t.Fatalf("the guitar tab has two versions, the row says %d", head.Count)
	}

	// a rating stands on the votes under it: five stars from two people is not
	// the best transcription of anything
	if head.Version != 2 {
		t.Fatalf("the row is the best liked version, got version %d", head.Version)
	}

	// the bass part is a row of its own and not a version of the guitar one.
	// it sits at the bottom, since a tab for the other instrument is somebody
	// else's part
	bass := m.results[len(m.results)-1]
	if bass.Kind != tabsite.KindBass || bass.Count != 1 {
		t.Fatalf("the bass part is its own row, got kind %q count %d",
			bass.Kind, bass.Count)
	}
}

// TestEnterOpensAndClosesTheVersions is the key on a row that has more than
// one transcription under it: which version to read is a question, and enter
// is what answers one.
func TestEnterOpensAndClosesTheVersions(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.showTabs([]tabsite.Result{
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 1, Rating: 4, Votes: 100, URL: "one"},
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 2, Rating: 4.8, Votes: 200, URL: "two"},
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 3, Rating: 4.6, Votes: 150, URL: "three"},
		{Artist: "Nobody", Title: "Another One", Kind: tabsite.KindTab, Version: 1, URL: "four"},
	})

	m.found = 0
	m.keySearch("enter")

	if len(m.results) != 5 {
		t.Fatalf("three versions under two songs is five rows, got %d", len(m.results))
	}
	if !m.results[0].Open {
		t.Fatal("the row the versions came out of does not say it is open")
	}

	// the versions are in the order people rate them, best first
	order := []int{m.results[1].Version, m.results[2].Version, m.results[3].Version}
	if order[0] != 2 || order[1] != 3 || order[2] != 1 {
		t.Fatalf("the versions are not in the order they are rated, got %v", order)
	}

	// the other song keeps its place under them
	if m.results[4].Title != "Another One" {
		t.Fatalf("the song under the group moved, row four is %q", m.results[4].Title)
	}

	// h closes the group from any of its versions, and the cursor comes back
	// to the row it was opened from
	m.found = 2
	m.keySearch("h")

	if len(m.results) != 2 {
		t.Fatalf("the group closed leaves two rows, got %d", len(m.results))
	}
	if m.found != 0 {
		t.Fatalf("the cursor is on the row that closed, it is on %d", m.found)
	}
	if m.results[0].Open {
		t.Fatal("the closed row still says it is open")
	}
	if m.focus != paneSearch {
		t.Fatal("h closed the group and left the column as well")
	}
}

// TestEnterOnASingleVersionReadsIt is the row of a song nobody wrote twice:
// there is nothing to open and the key means what it always did.
func TestEnterOnASingleVersionReadsIt(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.showTabs([]tabsite.Result{
		{Artist: "Nobody", Title: "Test Riff", Kind: tabsite.KindTab, Version: 1, URL: "one"},
	})

	if m.results[0].heads() {
		t.Fatal("a song with one transcription of it opens into nothing")
	}

	m.found = 0
	if cmd := m.keySearch("enter"); cmd == nil {
		t.Fatal("enter on the only version of a song reads it in")
	}
}

// TestEnterPractisesWhatIsAlreadyHere is the one meaning of the key on every
// list: play the song when it is here, and go and get it when it is not.
func TestEnterPractisesWhatIsAlreadyHere(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenMusic
	m.songs[0].Path = "/home/somebody/fretdeck/songs/test-riff.json"
	m.focus = paneSearch
	m.results = []finding{{Artist: "Nobody", Title: "Test Riff", Path: m.songs[0].Path}}
	m.current, m.engine, m.tab = nil, nil, nil

	m.enterSearch()

	if m.screen != screenPractice || m.current == nil {
		t.Fatalf("enter on a song that is here opens it, screen is %s", screenNames[m.screen])
	}
	if len(m.recent.Entries) != 1 {
		t.Fatalf("practising a song puts it at the top of the recent list, %d entries",
			len(m.recent.Entries))
	}
}

// TestLDoesNotOpenASongOnTheSearchScreen is the line between walking and
// deciding: l goes into a list, and enter is what starts a take.
func TestLDoesNotOpenASongOnTheSearchScreen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenMusic
	m.songs[0].Path = "/home/somebody/fretdeck/songs/test-riff.json"
	m.focus = paneSearch
	m.results = []finding{{Artist: "Nobody", Title: "Test Riff", Path: m.songs[0].Path}}
	m.current, m.engine, m.tab = nil, nil, nil

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	if m.screen != screenMusic || m.current != nil {
		t.Fatalf("l opened the song on the %s screen", screenNames[m.screen])
	}

	m.key(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenPractice || m.current == nil {
		t.Fatalf("enter did not open the song, the screen is %s", screenNames[m.screen])
	}
}

// TestTheNavigationLineSaysHowToMove is the line under the name: a button for
// every screen, and the key that walks each way against the side it walks
// towards, so the two belong to the strip and not to the window.
func TestTheNavigationLineSaysHowToMove(t *testing.T) {
	m := model(t)
	m.screen = screenSpotify

	lines := strings.Split(m.View(), "\n")
	line := strings.TrimRight(stripAnsi(lines[tabTop]), " ")
	bars := stripAnsi(lines[tabTop+1])

	for _, which := range tabScreens {
		if name := screenNames[which]; !strings.Contains(line, " "+name+" ") {
			t.Fatalf("%s is not a button of its own: %q", name, line)
		}
	}

	// a song open is not a button: it is the music screen with a song on it
	if strings.Contains(line, screenNames[screenPractice]) {
		t.Fatalf("the practice screen is on the navigation line: %q", line)
	}

	// the second row of a button is the bar under it, and the screen that is
	// open has the thick one
	for _, which := range tabScreens {
		name := screenNames[which]
		at := len([]rune(line[:strings.Index(line, name)]))
		if !strings.ContainsAny(string([]rune(bars)[at:at+1]), "─━") {
			t.Fatalf("%s has no bar under it: %q", name, bars)
		}
	}
	if !strings.Contains(bars, "━") {
		t.Fatalf("the screen that is open has no bar of its own: %q", bars)
	}

	if !strings.HasPrefix(strings.TrimLeft(line, " "), "[H]"+tabGap) {
		t.Fatalf("the key that walks back is not against the buttons: %q", line)
	}
	if !strings.HasSuffix(line, tabGap+"[L]") {
		t.Fatalf("the key that walks on is not against the buttons: %q", line)
	}

	// and the whole of it sits in the middle, give or take the odd column
	lead := len(line) - len(strings.TrimLeft(line, " "))
	if trail := m.width - lipgloss.Width(line); lead-trail > 1 || trail-lead > 1 {
		t.Fatalf("the line is not centred, %d cells before and %d after", lead, trail)
	}
}

// TestOnlyHAndLWalkTheScreens is the one way between them: a second scheme,
// the tab key or a number that jumps straight to a screen, is one more thing
// to remember for something two keys already do.
func TestOnlyHAndLWalkTheScreens(t *testing.T) {
	m := model(t)

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyTab},
		{Type: tea.KeyShiftTab},
		{Type: tea.KeyRunes, Runes: []rune{'3'}},
		{Type: tea.KeyRunes, Runes: []rune{'1'}},
	} {
		m.screen = screenMusic
		m.key(msg)

		if m.screen != screenMusic {
			t.Fatalf("%s walked to the %s screen", msg, screenNames[m.screen])
		}
	}

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if m.screen != screenSpotify {
		t.Fatalf("L did not walk on, the screen is %s", screenNames[m.screen])
	}

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if m.screen != screenMusic {
		t.Fatalf("H did not walk back, the screen is %s", screenNames[m.screen])
	}
}

// TestASongIsTheMusicScreenWithASongOnIt is why the practice screen is not a
// button: it is opened by opening a song, it lights the button the song came
// from, and backspace is the way back to the list.
func TestASongIsTheMusicScreenWithASongOnIt(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	if here := m.tabHere(); tabScreens[here] != screenMusic {
		t.Fatalf("a song open lit the %s button", screenNames[tabScreens[here]])
	}

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if m.screen != screenSpotify {
		t.Fatalf("L off a song walked to %s and not on from the music button", screenNames[m.screen])
	}

	m.screen = screenPractice
	m.key(tea.KeyMsg{Type: tea.KeyBackspace})

	if m.screen != screenMusic {
		t.Fatalf("backspace left the song for %s", screenNames[m.screen])
	}
}

// TestAKeyIsWrittenInBrackets is the one shape a key is offered in, on the bar
// at the bottom and in the help alike.
func TestAKeyIsWrittenInBrackets(t *testing.T) {
	for keys, want := range map[string]string{
		"i":        "[i]",
		"j k":      "[j/k]",
		"esc bksp": "[esc/bksp]",
		"[ ]":      "[ [ / ] ]",
	} {
		if got := keyLabel(keys); got != want {
			t.Fatalf("%q is written %q and not %q", keys, got, want)
		}
	}
}

// TestTheBarIsTheHelpKeyAndTheInput is the whole of the footer now: the map is
// behind one key and the line it used to fill is the list's.
func TestTheBarIsTheHelpKeyAndTheInput(t *testing.T) {
	m := model(t)
	m.screen = screenPractice
	m.status = "the click counts 120 bpm"

	lines := strings.Split(stripAnsi(m.footer()), "\n")
	if len(lines) != 2 {
		t.Fatalf("the footer is a rule and a line, it drew %d: %q", len(lines), lines)
	}

	line := lines[1]
	if !strings.HasPrefix(line, "[?] keys") {
		t.Fatalf("the footer does not open on the help key: %q", line)
	}

	// no key of the screen is written out there any more, and neither is what
	// the app had to say
	if strings.Contains(line, "[space]") || strings.Contains(line, "120 bpm") {
		t.Fatalf("the footer still carries the keys or the status: %q", line)
	}

	// the input keeps its corner, since it is read while nothing else is
	if !strings.Contains(line, "in ") {
		t.Fatalf("the input is not in the corner: %q", line)
	}
}

// TestTheHelpSaysEverythingTheScreenDoes is what the bar used to answer for:
// the map behind the question mark is the only place a key is written now, so
// every one of them has to be on it.
func TestTheHelpSaysEverythingTheScreenDoes(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	drawn := stripAnsi(m.viewHelp())
	for _, item := range m.bindings() {
		if !strings.Contains(drawn, keyLabel(item.keys)) {
			t.Fatalf("%s is on no line of the map: %q", keyLabel(item.keys), drawn)
		}
		if !strings.Contains(drawn, item.what) {
			t.Fatalf("%s is on the map with nothing beside it", keyLabel(item.keys))
		}
	}
}

// click is a press of the left button on a cell of the screen.
func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func wheel(down bool) tea.MouseMsg {
	button := tea.MouseButtonWheelUp
	if down {
		button = tea.MouseButtonWheelDown
	}
	return tea.MouseMsg{Action: tea.MouseActionPress, Button: button}
}

// TestClickingAButtonOpensThatScreen is the navigation line answering the
// mouse, on every row and column the button was drawn on, and not on the name
// alone.
func TestClickingAButtonOpensThatScreen(t *testing.T) {
	m := model(t)

	// the border is drawn in runes wider than a byte, so the column is counted
	line := stripAnsi(strings.Split(m.View(), "\n")[tabTop])
	at := len([]rune(line[:strings.Index(line, "spotify")]))

	// the bar under the name is the button as well, and so is the padding
	for row := tabTop; row < tabTop+tabRows; row++ {
		m.screen = screenMusic
		m.mouse(click(at+1, row))

		if m.screen != screenSpotify {
			t.Fatalf("clicking spotify on row %d opened %s", row, screenNames[m.screen])
		}
	}

	m.screen = screenMusic
	m.mouse(click(at-1, tabTop))
	if m.screen != screenSpotify {
		t.Fatalf("the padding of the button opened %s", screenNames[m.screen])
	}

	// what is between two buttons is no screen and opens nothing
	m.mouse(click(0, tabTop))
	if m.screen != screenSpotify {
		t.Fatalf("the left edge of the line is not a button, it opened %s", screenNames[m.screen])
	}
}

// TestClickingARowSelectsIt is the other half, and it selects without opening:
// the key that opens is the one on the bar.
func TestClickingARowSelectsIt(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = []finding{{Title: "one"}, {Title: "two"}, {Title: "three"}}

	// the rows are where the last drawing put them: in the right column, under
	// the search field and the head of the list
	m.View()
	column := m.sidebarWidth() + 4
	m.mouse(click(column, headerLines+len(m.searchBox(0))+2+2))

	if m.found != 2 {
		t.Fatalf("the third row was clicked, the cursor is on %d", m.found)
	}
	if m.screen != screenMusic {
		t.Fatal("a click selects a row, it does not open it")
	}

	// under the list is not a row
	m.mouse(click(column, m.height-2))
	if m.found != 2 {
		t.Fatalf("a click past the list moved the cursor to %d", m.found)
	}

	// the column answers as much as the line does: the same line on the left
	// is the list of what was played
	m.kept = []finding{{Title: "kept one"}, {Title: "kept two"}}
	m.View()
	m.mouse(click(2, headerLines+3))

	if m.focus != paneRecent {
		t.Fatal("a click on the left column left the keys on the search")
	}
}

// TestTheWheelScrollsTheListUnderIt is the same movement j and k make.
func TestTheWheelScrollsTheListUnderIt(t *testing.T) {
	m := model(t)
	m.screen = screenConfig
	m.configRow = 0

	m.mouse(wheel(true))
	if m.configRow != 1 {
		t.Fatalf("the wheel down did not move, the cursor is on %d", m.configRow)
	}

	m.mouse(wheel(false))
	if m.configRow != 0 {
		t.Fatalf("the wheel up did not come back, the cursor is on %d", m.configRow)
	}
}

// TestTheSiteCanBeSwitched is the whole of what the config row is for: the two
// sites answer differently and the one in force is what a search goes to.
func TestTheSiteCanBeSwitched(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenConfig
	m.results = []finding{{Title: "read off the other site"}}
	m.configRow = m.configCount() - len(tabsite.Sites) - 1

	kind, at := m.configPick()
	if kind != configSite || tabsite.Sites[at].Name != tabsite.Ultimate {
		t.Fatalf("the site list does not start where the cursor was put, got kind %d at %d", kind, at)
	}

	m.configRow++
	m.keepConfig()

	if m.cfg.Site != tabsite.Cifra {
		t.Fatalf("the site was not kept, config says %q", m.cfg.Site)
	}
	if config.Load().Site != tabsite.Cifra {
		t.Fatal("the answer did not survive the run")
	}
	if _, ok := m.site.(*cifraclub.Client); !ok {
		t.Fatalf("the search is still going to %T", m.site)
	}
	if len(m.results) != 0 {
		t.Fatal("the rows of the site that was left are pages this one cannot read")
	}
}

// TestAPageIsReadByTheSiteItCameFrom is the recent list: it keeps addresses
// from before the site was last changed, and neither site can read the
// other's page.
func TestAPageIsReadByTheSiteItCameFrom(t *testing.T) {
	chosen := openSite(tabsite.Cifra)

	if _, ok := siteOf("https://tabs.ultimate-guitar.com/tab/nobody/one-2", chosen).(*ultimate.Client); !ok {
		t.Fatal("an ultimate guitar address went to the site the config names")
	}
	if _, ok := siteOf("https://www.cifraclub.com.br/nobody/one/", chosen).(*cifraclub.Client); !ok {
		t.Fatal("a cifra club address went somewhere else")
	}
	if siteOf("https://example.invalid/one", chosen) != chosen {
		t.Fatal("an address of neither is read by the site in force, since that is where it was found")
	}
}

// TestTheMouseCanBeTurnedOff is why the switch is there: a terminal reporting
// the mouse is a terminal that no longer selects text with it.
func TestTheMouseCanBeTurnedOff(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenConfig
	m.cfg.Mouse = true
	m.configRow = m.configCount() - 1

	if kind, _ := m.configPick(); kind != configMouse {
		t.Fatalf("the last row of the config screen is the switch, got kind %d", kind)
	}

	m.keepConfig()

	if m.cfg.Mouse {
		t.Fatal("the switch did not turn the mouse off")
	}
	if m.Mouse() {
		t.Fatal("the program is started from what the config says")
	}
	if config.Load().Mouse {
		t.Fatal("the answer did not survive the run")
	}
}

// TestTheFlagsOpenTheAppWhereYouWereWorking is what make run passes on, and
// they are for working on the app: neither of them is kept.
func TestTheFlagsOpenTheAppWhereYouWereWorking(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	path := filepath.Join(dir, "riff.json")
	written, err := song.Write(dir, riff())
	if err != nil {
		t.Fatal(err)
	}
	path = written

	m := NewWith(Options{Song: path, Device: 7})

	if m.screen != screenPractice || m.current == nil {
		t.Fatalf("the song flag did not open it, the screen is %s and the fail %q",
			screenNames[m.screen], m.fail)
	}
	if m.cfg.Device != 7 {
		t.Fatalf("the device flag was not taken, the config says %d", m.cfg.Device)
	}
	if config.Load().Device == 7 {
		t.Fatal("a flag is for one run and is not kept")
	}

	// a path that does not read says why and leaves the app where it was
	broken := NewWith(Options{Song: filepath.Join(dir, "nothing.json"), Device: -1})
	if broken.screen == screenPractice || broken.fail == "" {
		t.Fatal("a song that does not read has to say so")
	}
}

// TestTheFirstRunWalksTwoSteps is what the config screen is until it has both
// answers: one question a screen, in order, and the app after them.
func TestTheFirstRunWalksTwoSteps(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New()
	m.width, m.height = 96, 30
	m.devices = []deviceInfo{{Index: 1, Name: "line in", Rate: 44100}}

	if m.first != firstRunInstrument || !strings.Contains(stripAnsi(m.View()), "step 1 of 2") {
		t.Fatal("the first step has to say which one it is")
	}

	// there is nothing to walk to yet, so the screen keys do nothing
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if m.screen != screenConfig {
		t.Fatalf("the first run was walked out of, onto %s", screenNames[m.screen])
	}

	m.keepConfig()
	if m.first != firstRunInput || !strings.Contains(stripAnsi(m.View()), "step 2 of 2") {
		t.Fatal("keeping the instrument has to open the second step")
	}

	// and the first answer can be changed while the run is still going
	m.back()
	if m.first != firstRunInstrument {
		t.Fatal("h did not go back a step")
	}

	m.keepConfig()
	m.keepConfig()

	if m.first != firstRunDone || m.screen != screenMusic {
		t.Fatalf("the app opens on the search once both are in, got %s", screenNames[m.screen])
	}
}

// TestTheFirstRunCanBeLeftForLater is the way out of a step that cannot be
// answered, such as an input list that came back empty.
func TestTheFirstRunCanBeLeftForLater(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New()
	m.width, m.height = 96, 30

	m.keyConfig("esc")

	if m.screen != screenMusic || m.first != firstRunDone {
		t.Fatalf("esc has to open the app anyway, got %s", screenNames[m.screen])
	}
	if config.Load().Instrument != "" {
		t.Fatal("nothing was answered, so nothing is kept and the next run asks again")
	}
}

// TestARefreshThatIsNotAnsweredSaysSo is the screen that used to read forever:
// a worker that dies between the asking and the answering says nothing at all.
func TestARefreshThatIsNotAnsweredSaysSo(t *testing.T) {
	m := model(t)
	m.screen = screenConfig
	m.listing = true

	m.Update(lateMsg{})

	if m.listing || m.fail == "" {
		t.Fatal("an answer that never came has to be said out loud")
	}

	// and a worker that goes while the app is running is not a silence either
	m.fail = ""
	m.handle(bridge.Event{Event: bridge.EventWorkerGone, Message: "the audio worker stopped"})

	if m.fail == "" || len(m.devices) != 0 {
		t.Fatalf("a dead worker left %d inputs on the screen and said %q", len(m.devices), m.fail)
	}
}

// TestTheRowUnderTheCursorIsPaintedAcross is the band the cursor is found by.
// It has to be under every piece of the row: what a row answers is written at
// the far edge of the screen, and a background laid over a finished line stops
// at the first reset the styles inside it wrote.
func TestTheRowUnderTheCursorIsPaintedAcross(t *testing.T) {
	// the profile is forced, since a runner with no colour on it would draw
	// the band and the plain row the same. zero is true colour
	before := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(before)

	m := &Model{width: 80}
	item := finding{Title: "Seven Nation Army", Artist: "The White Stripes",
		From: sourceSearch, Kind: tabsite.KindTab, Version: 4,
		Rating: 4.8, Votes: 1089, Level: 2, State: lookupDone}

	row := m.findingRow(item, true, m.width)
	if lipgloss.Width(row) != m.width {
		t.Fatalf("the row is %d columns wide, not %d", lipgloss.Width(row), m.width)
	}

	// the colour itself comes from the paint, so the palette stays the one
	// place a colour is written down
	probe := rowPaint(true).of(lipgloss.NewStyle()).Render("x")
	band := strings.TrimPrefix(probe[:strings.Index(probe, "x")], "\x1b[")

	if painted, ends := strings.Count(row, band), strings.Count(row, "\x1b[0m"); painted != ends {
		t.Fatalf("%d of the %d pieces of the row carry the band", painted, ends)
	}

	if plain := m.findingRow(item, false, m.width); strings.Contains(plain, band) {
		t.Fatal("a row the cursor is not on was painted too")
	}
}

// TestKWalksOffTheListOntoTheField is the top of the search column: the field
// is the row above the first result, and the cursor stops there.
func TestKWalksOffTheListOntoTheField(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = []finding{{Artist: "band", Title: "one"}, {Artist: "band", Title: "two"}}
	m.found = 1

	press(m, "k")
	if m.musicCursor() != 0 {
		t.Fatalf("k left the cursor at %d", m.musicCursor())
	}

	press(m, "k")
	if !m.onField() {
		t.Fatalf("k did not walk onto the field, cursor at %d", m.musicCursor())
	}

	press(m, "k")
	if !m.onField() {
		t.Fatal("k walked off the top of the column")
	}

	press(m, "j")
	if m.musicCursor() != 0 {
		t.Fatalf("j did not come back to the first row, cursor at %d", m.musicCursor())
	}
}

// TestEnterOnTheFieldStartsTyping is what the field being a row means: the key
// that answers a row opens the one it is on.
func TestEnterOnTheFieldStartsTyping(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = nil

	press(m, "enter")

	if !m.input.Focused() || m.mode != modeInsert {
		t.Fatalf("enter on the field left the app in the %s mode, focused %v", m.mode, m.input.Focused())
	}
}

// TestTheRecentRowSaysWhichSiteItCameFrom is the mark beside a played song:
// the two sites cannot read each other's pages, so which one answered for a
// song is part of the song.
func TestTheRecentRowSaysWhichSiteItCameFrom(t *testing.T) {
	m := model(t)

	from := m.recentRow(finding{Artist: "band", Title: "one",
		URL: "https://tabs.ultimate-guitar.com/tab/band/one-123"}, false, 36)
	if !strings.Contains(strings.Join(from, "\n"), "ug") {
		t.Fatalf("the row does not say where it came from: %q", from)
	}

	cifra := m.recentRow(finding{Artist: "band", Title: "one",
		URL: "https://www.cifraclub.com.br/band/one/"}, false, 36)
	if !strings.Contains(strings.Join(cifra, "\n"), "cc") {
		t.Fatalf("the row does not say where it came from: %q", cifra)
	}

	// a song imported from a file came off no site and is marked with nothing
	imported := strings.Join(m.recentRow(finding{Artist: "band", Title: "one",
		Path: "/songs/one.json"}, false, 36), "\n")
	if strings.Contains(imported, "ug") || strings.Contains(imported, "cc") {
		t.Fatalf("a song off no site was marked with one: %q", imported)
	}
}

// TestTheBarOffersWhatTheRowUnderTheCursorTakes is the one place a key is
// offered outside the help: the row being looked at says what can be done to
// it, and a column with nothing under the cursor says nothing.
func TestTheBarOffersWhatTheRowUnderTheCursorTakes(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.kept = []finding{{Artist: "band", Title: "one"}}

	if !strings.Contains(m.footer(), "delete music") {
		t.Fatalf("the bar does not offer d on a played song: %q", m.footer())
	}

	m.focus = paneSearch
	if strings.Contains(m.footer(), "delete music") {
		t.Fatal("the bar offers d on the column it does not belong to")
	}

	m.focus, m.kept = paneRecent, nil
	if strings.Contains(m.footer(), "delete music") {
		t.Fatal("the bar offers d with nothing under the cursor")
	}
}

// TestTheRemovalAsksOnTheBar is what d was missing: the question was set on a
// status nothing draws, so the key that answers it was never offered.
func TestTheRemovalAsksOnTheBar(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.kept = []finding{{Artist: "band", Title: "one", Path: "/songs/one.json"}}

	press(m, "d")

	if !m.removing {
		t.Fatal("d did not ask about the file")
	}

	bar := m.footer()
	if !strings.Contains(bar, "remove one?") || !strings.Contains(bar, "[y]") {
		t.Fatalf("the bar does not ask: %q", bar)
	}
}
