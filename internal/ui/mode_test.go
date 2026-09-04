package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press is one key the way the terminal sends it.
func press(m *Model, key string) {
	switch key {
	case "esc":
		m.key(tea.KeyMsg{Type: tea.KeyEsc})
	case "space":
		m.key(tea.KeyMsg{Type: tea.KeySpace})
	case "enter":
		m.key(tea.KeyMsg{Type: tea.KeyEnter})
	default:
		m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
}

// TestIStartsTypingAndEscComesBack is the insert mode: the search screen has
// one field and i is the key that opens it, the way / already did.
func TestIStartsTypingAndEscComesBack(t *testing.T) {
	m := model(t)
	m.screen = screenMusic

	press(m, "i")

	if m.mode != modeInsert {
		t.Fatalf("i left the app in the %s mode", m.mode)
	}
	if !m.input.Focused() || m.asking != askingQuery {
		t.Fatal("i did not open the search field")
	}

	press(m, "esc")

	if m.mode != modeNormal || m.input.Focused() {
		t.Fatalf("esc left the app in the %s mode with the field open: %v", m.mode, m.input.Focused())
	}
}

// TestTheSearchFieldIsOnTheMusicScreen is the section at the top of it: the
// field is drawn whether or not it has the cursor, i puts the cursor in it and
// enter is what searches. There is no slash any more.
func TestTheSearchFieldIsOnTheMusicScreen(t *testing.T) {
	m := model(t)
	m.screen = screenMusic

	if body := stripAnsi(m.View()); !strings.Contains(body, "SEARCH") {
		t.Fatalf("the music screen has no search section:\n%s", body)
	}

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m.input.Focused() {
		t.Fatal("the slash still opens the field")
	}

	press(m, "i")
	if m.mode != modeInsert || !m.input.Focused() {
		t.Fatalf("i left the app in the %s mode with the field %v", m.mode, m.input.Focused())
	}

	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("acdc")})
	if !strings.Contains(stripAnsi(m.View()), "acdc") {
		t.Fatal("what is being typed is not on the field")
	}

	m.key(tea.KeyMsg{Type: tea.KeyEnter})

	if m.query != "acdc" || m.input.Focused() {
		t.Fatalf("enter searched for %q with the field %v", m.query, m.input.Focused())
	}

	// what was searched stays on the field, so the screen says what the list
	// under it is answering
	if !strings.Contains(stripAnsi(m.View()), "acdc") {
		t.Fatal("the field forgot what was searched")
	}
}

// TestRPicksAPassageToRepeat is the repeat mode from the outside: r turns it
// on, space marks the measure under the cursor, and r again lets it go.
func TestRPicksAPassageToRepeat(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	press(m, "r")

	if m.mode != modeRepeat {
		t.Fatalf("r left the app in the %s mode", m.mode)
	}
	if accent != colorAzure {
		t.Fatal("the repeat mode did not paint the app blue")
	}

	press(m, "space")

	if !m.engine.Looping() || !m.engine.Repeats(1) {
		t.Fatal("space did not mark the measure under the cursor")
	}

	// the mark is on the tab as well, under the columns of that measure
	if band := stripAnsi(m.repeatBand(m.tab.View(m.engine.Cursor(), 60))); !strings.Contains(band, "━") {
		t.Fatalf("the marked measure is not drawn under the tab: %q", band)
	}

	press(m, "r")

	if m.mode != modeNormal || m.engine.Looping() {
		t.Fatalf("r a second time left the %s mode with a passage still picked", m.mode)
	}
	if accent != colorBrass {
		t.Fatal("leaving the repeat mode did not put the palette back")
	}
}

// TestSpaceDoesNothingOutsideTheRepeatMode is the promise the modes make: a
// key means what it always meant until a mode is turned on, and outside the
// repeat mode the space means nothing at all.
func TestSpaceDoesNothingOutsideTheRepeatMode(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	press(m, "space")

	if m.engine.Looping() {
		t.Fatal("space in the normal mode picked a passage")
	}
}

// TestLeavingThePracticeScreenLeavesTheRepeatMode is why the mode is the
// app's: its keys mean something else on every other screen.
func TestLeavingThePracticeScreenLeavesTheRepeatMode(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	press(m, "r")
	press(m, "space")
	press(m, "L")

	if m.mode != modeNormal {
		t.Fatalf("the %s mode walked to the %s screen", m.mode, screenNames[m.screen])
	}

	// what was picked is kept: it is the passage being practised, not a mode
	if !m.engine.Repeats(1) {
		t.Fatal("walking off the screen let the passage go")
	}
}
