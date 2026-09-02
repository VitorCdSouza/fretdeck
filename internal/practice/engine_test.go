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

func TestPulseCountsTheBarInItsOwnUnit(t *testing.T) {
	loaded := riff()
	loaded.Measures[0].Signature = [2]int{6, 8}
	engine := New(loaded, Tempo)
	origin := time.Now()
	engine.Start(origin)

	// at 120 bpm an eighth note is a quarter of a second, so three of them in
	// is the fourth pulse of a bar that holds six
	pulse, count := engine.Pulse(origin.Add(countIn).Add(750 * time.Millisecond))

	if count != 6 {
		t.Fatalf("a bar of six eight holds six pulses, got %d", count)
	}
	if pulse != 3 {
		t.Fatalf("want the fourth pulse, got %d", pulse)
	}
}

func TestPulseWrapsAtTheBarLine(t *testing.T) {
	engine := New(riff(), Tempo)
	origin := time.Now()
	engine.Start(origin)

	// four quarters at 120 bpm is two seconds, which is the next bar line
	pulse, count := engine.Pulse(origin.Add(countIn).Add(2 * time.Second))

	if count != 4 || pulse != 0 {
		t.Fatalf("want the first pulse of four, got pulse %d of %d", pulse, count)
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
	engine := New(passage(), Wait)
	engine.ToggleRepeat(1)
	now := time.Now()

	engine.Heard(43, now)
	engine.Heard(45, now)

	if engine.Cursor() != 0 {
		t.Fatalf("the passage did not come round, the cursor is at %d", engine.Cursor())
	}
	if engine.Passes() != 1 {
		t.Fatalf("one pass was played, it counted %d", engine.Passes())
	}

	// the verdicts of the pass that ended go with it, so the accuracy on the
	// screen is about the take being played now
	if engine.Result(0).Verdict != Pending {
		t.Fatalf("the last pass was left on the tab, note one is %v", engine.Result(0).Verdict)
	}
}

// TestPickingAPassageBehindTheCursorGoesBackToIt is what makes the mark do
// something the moment it is made.
func TestPickingAPassageBehindTheCursorGoesBackToIt(t *testing.T) {
	engine := New(passage(), Wait)
	engine.Seek(3)

	engine.ToggleRepeat(1)
	engine.Loop(time.Now())

	if engine.Cursor() != 0 {
		t.Fatalf("the cursor is at %d and not on the passage", engine.Cursor())
	}
}

// TestLettingThePassageGoRunsTheSongToItsEnd is the way back out.
func TestLettingThePassageGoRunsTheSongToItsEnd(t *testing.T) {
	engine := New(passage(), Wait)
	engine.ToggleRepeat(1)
	engine.ClearRepeat()
	now := time.Now()

	engine.Heard(43, now)
	engine.Heard(45, now)

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
	engine := New(passage(), Wait)
	engine.ToggleRepeat(2)
	engine.Loop(time.Now())

	if engine.Cursor() != 2 {
		t.Fatalf("the second measure begins at event 2, the cursor is at %d", engine.Cursor())
	}

	now := time.Now()
	engine.Heard(47, now)
	engine.Heard(48, now)

	if engine.Cursor() != 2 {
		t.Fatalf("the passage did not come round, the cursor is at %d", engine.Cursor())
	}
}

// TestTheClockComesBackWithThePassage is what keeps the tempo mode honest: a
// loop that left the clock where it was would write the passage off as missed
// the instant it started again.
func TestTheClockComesBackWithThePassage(t *testing.T) {
	engine := New(passage(), Tempo)
	origin := time.Now()
	engine.Start(origin)
	engine.ToggleRepeat(1)

	// far enough in that both notes of the first measure are gone
	now := origin.Add(countIn + 3*time.Second)
	engine.Tick(now)

	if engine.Cursor() != 0 {
		t.Fatalf("the passage did not come round, the cursor is at %d", engine.Cursor())
	}
	if engine.CountIn(now) <= 0 {
		t.Fatal("the passage came round with no count in, so it starts under the hand")
	}
	if elapsed := engine.Elapsed(now); elapsed > 0 {
		t.Fatalf("the clock is %s into a passage that has not started", elapsed)
	}
}
