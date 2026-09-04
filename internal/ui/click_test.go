package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
)

func TestTheClickKeyTurnsItOnAndSaysTheBeat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenPractice

	m.keyPractice("c")
	if !m.engine.ClickOn() {
		t.Fatal("c did not start the click")
	}
	if !m.cfg.Click {
		t.Fatal("the click was not kept for the next run")
	}
	if !strings.Contains(m.status, "bpm") {
		t.Fatalf("the beat was not said: %q", m.status)
	}

	m.keyPractice("c")
	if m.engine.ClickOn() {
		t.Fatal("c a second time did not stop it")
	}
}

// the command only goes down the pipe when the answer changes, since it is
// worked out twenty five times a second.
func TestTheBeatIsOnlySentWhenItChanges(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenPractice
	now := time.Now()

	if cmd := m.count(now); cmd != nil {
		t.Fatal("a click nobody asked for was sent")
	}

	m.engine.StartClick(now)
	if cmd := m.count(now); cmd == nil {
		t.Fatal("the click was turned on and nothing was sent")
	}
	if cmd := m.count(now); cmd != nil {
		t.Fatal("the same beat was sent twice")
	}

	// walking away from the song is the click going quiet, and that is a change
	m.screen = screenMusic
	if cmd := m.count(now); cmd == nil {
		t.Fatal("leaving the practice screen left the click counting")
	}
}

func TestTheDifficultyKeyWalksTheLadderAndRedrawsTheTab(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenPractice
	m.current.Notes[0].Technique = song.Bend
	m.engine.SetLevel(song.Full)
	m.tab = song.NewTab(m.current, m.engine.Events)

	drawn := func() string {
		view := m.tab.View(0, 60)
		return view.Rows[len(view.Rows)-1].At + view.Rows[len(view.Rows)-1].After
	}

	if !strings.Contains(drawn(), "b") {
		t.Fatalf("full does not draw the bend: %q", drawn())
	}

	// full is the top of the ladder, so the next one round is the bottom
	m.keyPractice("d")
	if m.engine.Level() != song.Plain {
		t.Fatalf("want plain, got %q", m.engine.Level())
	}
	if strings.Contains(drawn(), "b") {
		t.Fatalf("plain still draws the bend: %q", drawn())
	}
	if m.cfg.Level != "plain" {
		t.Fatalf("the level was not kept, the config says %q", m.cfg.Level)
	}
}

func TestATypedBeatIsTheSpeedOfTheSong(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenPractice
	m.engine.Mode = practice.Tempo

	m.setBpm("60")

	if got := m.engine.Bpm(); got != 60 {
		t.Fatalf("want 60, got %.0f", got)
	}
	want := 60 / m.current.Tempo
	if got := m.engine.Speed; got != want {
		t.Fatalf("60 of a written %.0f is %.2f, got %.2f", m.current.Tempo, want, got)
	}
	if m.cfg.Speed != want || m.cfg.Bpm != 60 {
		t.Fatalf("the beat was not kept: %.2f and %.0f", m.cfg.Speed, m.cfg.Bpm)
	}
}

func TestABeatThatIsNotANumberSaysSoAndChangesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenPractice

	m.setBpm("quickly")

	if m.fail == "" {
		t.Fatal("nothing was said about a beat that is not a number")
	}
	if m.engine.Speed != 1 {
		t.Fatalf("the speed moved to %.2f", m.engine.Speed)
	}
}

// the bar at the bottom and the help overlay are drawn from the one list, so a
// key that is not in it is a key nobody finds.
func TestTheNewKeysAreOnTheMap(t *testing.T) {
	m := model(t)
	m.screen = screenPractice

	found := map[string]bool{}
	for _, item := range m.bindings() {
		found[item.keys] = true
	}

	for _, key := range []string{"b", "c", "d"} {
		if !found[key] {
			t.Fatalf("%q is not on the key map", key)
		}
	}
}

// the app has one text field, so a screen that opens it has to draw it: a beat
// typed into a line nobody can see is a beat typed blind.
func TestTheFieldIsDrawnOnTheScreenThatOpenedIt(t *testing.T) {
	m := model(t)
	m.screen = screenPractice
	m.input.Width = 40

	m.keyPractice("b")
	if !m.input.Focused() {
		t.Fatal("b did not open the field")
	}

	m.input.SetValue("140")
	if !strings.Contains(m.viewPractice(), "140") {
		t.Fatal("the field is open and the practice screen does not draw it")
	}
}
