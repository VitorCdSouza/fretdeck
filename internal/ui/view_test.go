package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/config"
	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

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
	engine := practice.New(loaded, practice.Wait)

	return &Model{
		cfg:     config.Config{Device: 1, Rate: 44100, Library: "/home/somebody/fretdeck/songs", Speed: 1},
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
	m.engine.Heard(44, time.Now())

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

// TestTheHelpAndTheHighwayFitTheWindowToo covers the two views that are not a
// screen of their own and so are missed by the loop above.
func TestTheHelpAndTheHighwayFitTheWindowToo(t *testing.T) {
	m := model(t)

	m.screen = screenPractice
	m.highway = true
	mustFit(t, m, "highway")

	m.highway = false
	m.helping = true
	mustFit(t, m, "help")
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

// TestFilterNarrowsTheLibrary is the slash key. It matches over the three
// things on the row, since any of them is a reasonable thing to type.
func TestFilterNarrowsTheLibrary(t *testing.T) {
	m := model(t)
	m.songs = append(m.songs, &song.Song{Title: "Another One", Artist: "Somebody", Track: "Bass"})

	for _, needle := range []string{"test", "NOBODY", "guitar 1"} {
		m.filter = needle
		if got := len(m.filtered()); got != 1 {
			t.Fatalf("filter %q kept %d songs, want the one", needle, got)
		}
	}

	m.filter = "nothing like this"
	if got := len(m.filtered()); got != 0 {
		t.Fatalf("want an empty list, got %d", got)
	}
}

// TestMovementIsVimOnEveryList is the promise the help makes.
func TestMovementIsVimOnEveryList(t *testing.T) {
	m := model(t)
	m.screen = screenSearch
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
	m.screen = screenLibrary

	m.move(1)
	m.jump(1)

	if m.pick != 0 {
		t.Fatalf("want the cursor at zero, got %d", m.pick)
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
