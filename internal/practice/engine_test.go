package practice

import (
	"testing"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

func riff() *song.Song {
	return &song.Song{
		Title:  "riff",
		Tempo:  120,
		Tuning: []song.String{{Number: 1, Midi: 64}, {Number: 6, Midi: 40}},
		Measures: []song.Measure{
			{Index: 1, Beat: 0, Time: 0, Tempo: 120, Signature: [2]int{4, 4}},
		},
		Notes: []song.Note{
			{Measure: 1, Beat: 0, Time: 0, Dur: 0.5, String: 6, Fret: 3, Midi: 43},
			{Measure: 1, Beat: 1, Time: 0.5, Dur: 0.5, String: 6, Fret: 5, Midi: 45},
			{Measure: 1, Beat: 2, Time: 1, Dur: 0.5, String: 6, Fret: 7, Midi: 47},
		},
	}
}

func TestTheCursorHoldsOnAWrongNote(t *testing.T) {
	engine := New(riff())

	if got := engine.Heard(44); got != Wrong {
		t.Fatalf("want Wrong, got %v", got)
	}
	if engine.Cursor() != 0 {
		t.Fatalf("a wrong note cannot move the cursor, it is at %d", engine.Cursor())
	}

	if got := engine.Heard(43); got != Hit {
		t.Fatalf("want Hit, got %v", got)
	}
	if engine.Cursor() != 1 {
		t.Fatalf("want the cursor on the second note, it is at %d", engine.Cursor())
	}
}

// TestTheLowStringsReadAnOctaveUp is what keeps somebody from being failed for
// playing the note they were asked for.
func TestTheLowStringsReadAnOctaveUp(t *testing.T) {
	engine := New(riff())

	if got := engine.Heard(43 + 12); got != Hit {
		t.Fatalf("the second harmonic of a wound string is a hit, got %v", got)
	}
}

// TestTheCursorStopsOnTheLastNote is what keeps the caret and the measure
// number about the same place: a cursor past the end draws the tab from the
// top while the head still reads the last measure of the song.
func TestTheCursorStopsOnTheLastNote(t *testing.T) {
	engine := New(riff())
	last := len(engine.Events) - 1

	engine.Seek(len(engine.Events) + 5)
	if engine.Cursor() != last {
		t.Fatalf("want the cursor on the last note, it is at %d", engine.Cursor())
	}

	engine.Heard(43)
	engine.Heard(45)
	engine.Heard(47)
	if engine.Cursor() != last {
		t.Fatalf("playing the song out left the cursor at %d", engine.Cursor())
	}
	if _, playing := engine.Current(); !playing {
		t.Fatal("the last note is still a note to play")
	}
}

// TestAPassageOnTheLastMeasureStillComesRound is the other half of that: the
// step past the end is what the loop wraps from.
func TestAPassageOnTheLastMeasureStillComesRound(t *testing.T) {
	engine := New(passage())
	engine.ToggleRepeat(2)
	engine.Loop()

	engine.Heard(47)
	engine.Heard(48)

	if engine.Cursor() != 2 {
		t.Fatalf("the last measure did not come round, the cursor is at %d", engine.Cursor())
	}
}

func TestSeekMeasureLandsOnItsFirstNote(t *testing.T) {
	engine := New(riff())
	engine.SeekMeasure(1)

	if engine.Cursor() != 0 {
		t.Fatalf("want the first note of the measure, got %d", engine.Cursor())
	}
}

// passage is two measures, so one of them can be picked and the other left out
// of the loop.
func passage() *song.Song {
	return &song.Song{
		Title:  "passage",
		Tempo:  120,
		Tuning: []song.String{{Number: 1, Midi: 64}, {Number: 6, Midi: 40}},
		Measures: []song.Measure{
			{Index: 1, Beat: 0, Time: 0, Tempo: 120, Signature: [2]int{4, 4}},
			{Index: 2, Beat: 4, Time: 2, Tempo: 120, Signature: [2]int{4, 4}},
		},
		Notes: []song.Note{
			{Measure: 1, Beat: 0, Time: 0, Dur: 0.5, String: 6, Fret: 3, Midi: 43},
			{Measure: 1, Beat: 1, Time: 0.5, Dur: 0.5, String: 6, Fret: 5, Midi: 45},
			{Measure: 2, Beat: 4, Time: 2, Dur: 0.5, String: 6, Fret: 7, Midi: 47},
			{Measure: 2, Beat: 5, Time: 2.5, Dur: 0.5, String: 6, Fret: 8, Midi: 48},
		},
	}
}

// TestAPickedPassageComesRoundAgain is the whole of the repeat: the last note
// of the passage is followed by its first and not by the rest of the song.
func TestAPickedPassageComesRoundAgain(t *testing.T) {
	engine := New(passage())
	engine.ToggleRepeat(1)

	engine.Heard(43)
	engine.Heard(45)

	if engine.Cursor() != 0 {
		t.Fatalf("the passage did not come round, the cursor is at %d", engine.Cursor())
	}
	if engine.Passes() != 1 {
		t.Fatalf("one pass was played, it counted %d", engine.Passes())
	}

	// the verdicts of the pass that ended go with it, so what the tab is
	// painted from is about the take being played now
	if engine.Result(0).Verdict != Pending {
		t.Fatalf("the last pass was left on the tab, note one is %v", engine.Result(0).Verdict)
	}
}

// TestPickingAPassageBehindTheCursorGoesBackToIt is what makes the mark do
// something the moment it is made.
func TestPickingAPassageBehindTheCursorGoesBackToIt(t *testing.T) {
	engine := New(passage())
	engine.Seek(3)

	engine.ToggleRepeat(1)
	engine.Loop()

	if engine.Cursor() != 0 {
		t.Fatalf("the cursor is at %d and not on the passage", engine.Cursor())
	}
}

// TestLettingThePassageGoRunsTheSongToItsEnd is the way back out.
func TestLettingThePassageGoRunsTheSongToItsEnd(t *testing.T) {
	engine := New(passage())
	engine.ToggleRepeat(1)
	engine.ClearRepeat()

	engine.Heard(43)
	engine.Heard(45)

	if engine.Cursor() != 2 {
		t.Fatalf("the song did not go on, the cursor is at %d", engine.Cursor())
	}
	if engine.Looping() {
		t.Fatal("the passage was let go and something is still being repeated")
	}
}

// TestOnlyThePickedMeasuresAreLoopedOver is the other half: a measure nobody
// picked is not part of the passage.
func TestOnlyThePickedMeasuresAreLoopedOver(t *testing.T) {
	engine := New(passage())
	engine.ToggleRepeat(2)
	engine.Loop()

	if engine.Cursor() != 2 {
		t.Fatalf("the second measure begins at event 2, the cursor is at %d", engine.Cursor())
	}

	engine.Heard(47)
	engine.Heard(48)

	if engine.Cursor() != 2 {
		t.Fatalf("the passage did not come round, the cursor is at %d", engine.Cursor())
	}
}
