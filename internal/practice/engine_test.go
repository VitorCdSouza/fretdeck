package practice

import (
	"testing"
	"time"

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

func TestWaitModeHoldsTheCursorOnAWrongNote(t *testing.T) {
	engine := New(riff(), Wait)
	now := time.Now()

	if got := engine.Heard(44, now); got != Wrong {
		t.Fatalf("want Wrong, got %v", got)
	}
	if engine.Cursor() != 0 {
		t.Fatalf("a wrong note cannot move the cursor, it is at %d", engine.Cursor())
	}

	if got := engine.Heard(43, now); got != Hit {
		t.Fatalf("want Hit, got %v", got)
	}
	if engine.Cursor() != 1 {
		t.Fatalf("want the cursor on the second note, it is at %d", engine.Cursor())
	}
}

func TestWaitModeIgnoresTheClock(t *testing.T) {
	engine := New(riff(), Wait)
	engine.Start(time.Now())
	engine.Tick(time.Now().Add(time.Hour))

	if engine.Cursor() != 0 {
		t.Fatal("wait mode has no moment to miss, so an hour cannot skip a note")
	}
}

func TestTempoModeCountsANoteOnTime(t *testing.T) {
	engine := New(riff(), Tempo)
	origin := time.Now()
	engine.Start(origin)

	// the count in comes first, so the clock of the song starts two seconds
	// after the screen does
	at := origin.Add(countIn).Add(20 * time.Millisecond)
	if got := engine.Heard(43, at); got != Hit {
		t.Fatalf("want Hit, got %v", got)
	}

	score := engine.Score()
	if score.Hits != 1 {
		t.Fatalf("want one hit, got %d", score.Hits)
	}
	if score.AvgOffset < 15*time.Millisecond || score.AvgOffset > 25*time.Millisecond {
		t.Fatalf("the offset should be around 20 ms, it is %v", score.AvgOffset)
	}
}

func TestTempoModeWritesOffTheNoteNobodyPlayed(t *testing.T) {
	engine := New(riff(), Tempo)
	origin := time.Now()
	engine.Start(origin)

	engine.Tick(origin.Add(countIn).Add(400 * time.Millisecond))

	if engine.Result(0).Verdict != Missed {
		t.Fatalf("the first note went by unplayed, got %v", engine.Result(0).Verdict)
	}
	if engine.Cursor() != 1 {
		t.Fatalf("want the cursor past the missed note, it is at %d", engine.Cursor())
	}
}

func TestTempoModeTakesANotePlayedAhead(t *testing.T) {
	engine := New(riff(), Tempo)
	origin := time.Now()
	engine.Start(origin)

	// the first note on time, the second one early enough that the clock has
	// not reached it yet
	engine.Heard(43, origin.Add(countIn))
	if got := engine.Heard(45, origin.Add(countIn).Add(400*time.Millisecond)); got != Hit {
		t.Fatalf("a note played inside the window before the beat is a hit, got %v", got)
	}
}

func TestSpeedStretchesTheClock(t *testing.T) {
	engine := New(riff(), Tempo)
	engine.Speed = 0.5
	origin := time.Now()
	engine.Start(origin)

	// at half speed one second of the room is half a second of the song
	if elapsed := engine.Elapsed(origin.Add(countIn).Add(time.Second)); elapsed != 500*time.Millisecond {
		t.Fatalf("want 500ms of song, got %v", elapsed)
	}
}

func TestScoreOnlyCountsWhatWasPlayed(t *testing.T) {
	engine := New(riff(), Wait)
	now := time.Now()
	engine.Heard(43, now)

	score := engine.Score()
	if score.Total != 1 {
		t.Fatalf("the two notes ahead are not judged yet, total is %d", score.Total)
	}
	if score.Accuracy != 1 {
		t.Fatalf("want a clean run so far, got %v", score.Accuracy)
	}
}

func TestSeekMeasureLandsOnItsFirstNote(t *testing.T) {
	engine := New(riff(), Wait)
	engine.SeekMeasure(1)

	if engine.Cursor() != 0 {
		t.Fatalf("want the first note of the measure, got %d", engine.Cursor())
	}
}
