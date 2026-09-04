package practice

import (
	"testing"
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// text is what an ascii tab off the internet comes in as: the notes and their
// order, and no tempo at all to take a beat from.
func text() *song.Song {
	loaded := riff()
	loaded.Untimed = true
	loaded.Tempo = 0
	return loaded
}

func TestABeatIsTheWrittenTempoScaled(t *testing.T) {
	engine := New(riff(), Tempo)

	if got := engine.Bpm(); got != 120 {
		t.Fatalf("the song is written at 120, got %.0f", got)
	}

	engine.SetSpeed(0.5)
	if got := engine.Bpm(); got != 60 {
		t.Fatalf("half of 120 is 60, got %.0f", got)
	}
}

func TestAskingForABeatIsAskingForASpeed(t *testing.T) {
	engine := New(riff(), Tempo)

	if got := engine.SetBpm(90); got != 90 {
		t.Fatalf("want 90, got %.0f", got)
	}
	if got := engine.Speed; got != 0.75 {
		t.Fatalf("90 of a written 120 is three quarters, got %.2f", got)
	}
}

func TestABeatThePieceCannotRunAtIsCutBackAndSaidSo(t *testing.T) {
	engine := New(riff(), Tempo)

	got := engine.SetBpm(600)
	if got != 120*MaxSpeed {
		t.Fatalf("the speed has an end, want %.0f, got %.0f", 120*MaxSpeed, got)
	}
	if engine.Speed != MaxSpeed {
		t.Fatalf("the speed ran past its end at %.2f", engine.Speed)
	}
}

func TestATabWithNoTempoKeepsABeatOfItsOwn(t *testing.T) {
	engine := New(text(), Wait)

	if got := engine.Bpm(); got != defaultBpm {
		t.Fatalf("a tab with no tempo opens on %.0f, got %.0f", defaultBpm, got)
	}

	// the song is not run by this, so the speed is left where it was: there is
	// no written tempo for it to be a multiple of
	engine.SetBpm(80)
	if got := engine.Bpm(); got != 80 {
		t.Fatalf("want 80, got %.0f", got)
	}
	if engine.Speed != 1 {
		t.Fatalf("nothing scales a song with no tempo, the speed is %.2f", engine.Speed)
	}
}

func TestTheClickCountsTheBarOnItsOwnClockInWaitMode(t *testing.T) {
	engine := New(text(), Wait)
	engine.SetBpm(120)

	now := time.Now()
	engine.StartClick(now)

	// half a second a beat, and the bar is the four four the song opens on
	for beat, at := range []time.Duration{0, 500, 1000, 1500, 2000} {
		pulse, count := engine.ClickPulse(now.Add(at * time.Millisecond))
		if count != 4 {
			t.Fatalf("want a bar of four, got %d", count)
		}
		if pulse != beat%4 {
			t.Fatalf("at %v want pulse %d, got %d", at*time.Millisecond, beat%4, pulse)
		}
	}
}

func TestTheClickIsTheSongsOwnBeatWhileTheSongIsRunning(t *testing.T) {
	engine := New(riff(), Tempo)
	now := time.Now()

	engine.StartClick(now)
	engine.Start(now)

	// the count in is over by then, and the second beat of the bar is where a
	// song at 120 is half a second after it starts
	at := now.Add(countIn + 500*time.Millisecond)

	pulse, count := engine.ClickPulse(at)
	if count != 4 || pulse != 1 {
		t.Fatalf("want pulse 1 of 4, got %d of %d", pulse, count)
	}

	// and it is the song's own clock and not the click's, which was started a
	// count in earlier and would be somewhere else by now
	own, _ := engine.Pulse(at)
	if own != pulse {
		t.Fatalf("the two counts disagree: %d and %d", own, pulse)
	}
}

func TestTheClickFollowsATempoChange(t *testing.T) {
	loaded := riff()
	loaded.Measures = append(loaded.Measures, song.Measure{
		Index: 2, Beat: 4, Time: 2, Tempo: 90, Signature: [2]int{3, 4},
	})

	engine := New(loaded, Tempo)
	now := time.Now()
	engine.Start(now)
	engine.StartClick(now)

	bpm, beats := engine.Click(now.Add(countIn))
	if bpm != 120 || beats != 4 {
		t.Fatalf("the song opens at 120 in four, got %.0f in %d", bpm, beats)
	}

	bpm, beats = engine.Click(now.Add(countIn + 2500*time.Millisecond))
	if bpm != 90 || beats != 3 {
		t.Fatalf("the second measure is 90 in three, got %.0f in %d", bpm, beats)
	}
}

func TestALevelChangesWhatIsAskedForAndNothingElse(t *testing.T) {
	loaded := riff()
	loaded.Notes[1].Technique = song.Bend

	engine := New(loaded, Wait)
	now := time.Now()

	engine.Heard(43, now)
	if engine.Cursor() != 1 {
		t.Fatalf("the first note was played, the cursor is at %d", engine.Cursor())
	}

	engine.SetLevel(song.Plain)

	if engine.Cursor() != 1 {
		t.Fatalf("a level moved the cursor to %d", engine.Cursor())
	}
	if engine.Result(0).Verdict != Hit {
		t.Fatal("a level threw away a verdict already given")
	}
	if got := engine.Events[1].Notes[0].Technique; got != "" {
		t.Fatalf("plain still asks for %q", got)
	}
	if got := engine.Events[1].Notes[0].Midi; got != 45 {
		t.Fatalf("a level moved the note to midi %d", got)
	}
}
