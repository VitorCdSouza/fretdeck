package ui

import (
	"strings"
	"testing"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/config"
)

func metronome(t *testing.T) *Model {
	t.Helper()

	m := model(t)
	m.screen = screenMetronome
	m.metro = metroBeat
	return m
}

// TestSpaceStartsTheClickAndStopsIt is the one key the screen is for, and it
// answers from whichever row the cursor is on.
func TestSpaceStartsTheClickAndStopsIt(t *testing.T) {
	m := metronome(t)
	m.metro = metroBar

	press(m, "space")

	if !m.clicking {
		t.Fatal("space did not start the click")
	}
	if command := m.clickCommand(); command.Bpm != m.cfg.Bpm || command.Beats != m.cfg.Beats {
		t.Fatalf("the worker was told %v, the screen says %.0f bpm and %d to the bar",
			command, m.cfg.Bpm, m.cfg.Beats)
	}

	press(m, "space")

	if m.clicking {
		t.Fatal("space a second time did not stop the click")
	}
	// a beat of nothing is the whole of turning it off
	if command := m.clickCommand(); command.Bpm != 0 || command.Beats != m.cfg.Beats {
		t.Fatalf("stopping the click sent %v", command)
	}
}

// TestTheBeatStopsAtTheEndsOfWhatCanBePractised is why the two ends are in the
// config and not in the screen: a file can carry a beat as well as a key can.
func TestTheBeatStopsAtTheEndsOfWhatCanBePractised(t *testing.T) {
	m := metronome(t)

	for range 40 {
		press(m, "]")
	}
	if m.cfg.Bpm != config.MaxBpm {
		t.Fatalf("the beat went to %.0f, the end of it is %.0f", m.cfg.Bpm, config.MaxBpm)
	}

	for range 40 {
		press(m, "[")
	}
	if m.cfg.Bpm != config.MinBpm {
		t.Fatalf("the beat went to %.0f, the end of it is %.0f", m.cfg.Bpm, config.MinBpm)
	}
}

// TestTheBracketKeysMoveTheBeatByTen is the key held down that a hundred and
// forty from sixty would otherwise be.
func TestTheBracketKeysMoveTheBeatByTen(t *testing.T) {
	m := metronome(t)
	m.cfg.Bpm = 90

	press(m, "]")
	if m.cfg.Bpm != 100 {
		t.Fatalf("] took 90 to %.0f", m.cfg.Bpm)
	}

	press(m, "l")
	if m.cfg.Bpm != 101 {
		t.Fatalf("l took 100 to %.0f", m.cfg.Bpm)
	}
}

// TestHAndLAreOffAndOnOnTheClickRow is the one row of the three that is not a
// number: less and more of a switch is off and on.
func TestHAndLAreOffAndOnOnTheClickRow(t *testing.T) {
	m := metronome(t)
	m.metro = metroClick

	press(m, "l")
	if !m.clicking {
		t.Fatal("l on the click row did not start it")
	}

	press(m, "h")
	if m.clicking {
		t.Fatal("h on the click row did not stop it")
	}
	if m.cfg.Bpm != 90 {
		t.Fatalf("the click row moved the beat to %.0f", m.cfg.Bpm)
	}
}

// TestTheBarIsAtLeastOneBeat is the other end of the same rule. A bar of none
// is a click with nothing to accent.
func TestTheBarIsAtLeastOneBeat(t *testing.T) {
	m := metronome(t)
	m.metro = metroBar

	for range 20 {
		press(m, "h")
	}
	if m.cfg.Beats != config.MinBeats {
		t.Fatalf("the bar went to %d beats", m.cfg.Beats)
	}

	for range 40 {
		press(m, "l")
	}
	if m.cfg.Beats != config.MaxBeats {
		t.Fatalf("the bar went to %d beats", m.cfg.Beats)
	}
}

// TestTheBeatIsKeptButTheClickIsNot is what a run opens on: the numbers that
// were answered, and silence, since a click counting into an empty room is
// something somebody has to turn off before anything else.
func TestTheBeatIsKeptButTheClickIsNot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := metronome(t)
	m.metro = metroBar
	press(m, "l")
	m.metro = metroBeat
	press(m, "]")
	press(m, "space")

	kept := config.Load()
	if kept.Bpm != m.cfg.Bpm || kept.Beats != m.cfg.Beats {
		t.Fatalf("the config kept %.0f bpm and %d to the bar, the screen says %.0f and %d",
			kept.Bpm, kept.Beats, m.cfg.Bpm, m.cfg.Beats)
	}

	again := &Model{cfg: kept}
	if again.clicking {
		t.Fatal("a run opened counting")
	}
}

// TestAWorkerThatDiedLeavesNothingCounting is the click as this side believes
// it to be: the count is the worker's, so a worker that is gone is a screen
// that has to stop saying it is counting.
func TestAWorkerThatDiedLeavesNothingCounting(t *testing.T) {
	m := metronome(t)
	press(m, "space")

	m.handle(bridge.Event{Event: bridge.EventWorkerGone, Message: "the audio worker stopped"})

	if m.clicking {
		t.Fatal("the worker is gone and the screen still says it is counting")
	}
}

// TestAnOutputThatCannotBePlayedOnSaysSo is the click and not the listening:
// the worker stops asking for an output and the screen stops promising one.
func TestAnOutputThatCannotBePlayedOnSaysSo(t *testing.T) {
	m := metronome(t)
	press(m, "space")

	m.handle(bridge.Event{Event: bridge.EventClickError, Message: "no output"})

	if m.clicking {
		t.Fatal("the click could not be played and the screen still says it is counting")
	}
	if m.fail != "no output" {
		t.Fatalf("the reason was not said, the bar reads %q", m.fail)
	}
}

// TestTheScreenSaysTheBeatTheBarAndTheState is the whole of what it draws:
// there is no count on it, since a second one would disagree with the one
// being heard inside a bar.
func TestTheScreenSaysTheBeatTheBarAndTheState(t *testing.T) {
	m := metronome(t)
	m.cfg.Bpm, m.cfg.Beats = 120, 3

	view := m.viewMetronome()
	for _, want := range []string{"120 bpm", "3 beats", "off"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the screen does not say %q", want)
		}
	}

	press(m, "space")
	if !strings.Contains(m.viewMetronome(), "counting") {
		t.Fatal("the screen does not say the click is counting")
	}
}
