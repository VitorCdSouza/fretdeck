package practice

import (
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// The metronome is the one clock here that is not the song's.
//
// Wait mode has no clock at all and a text tab has no tempo to take one from,
// and a beat to play against is what both of those are missing. In tempo mode
// it is the song's own beat instead: two counts running side by side would
// disagree within a bar, and the one that is wrong is the one nobody can tell
// from the right one.
//
// The engine still owns no clock of its own. The caller hands it the moment
// the click was turned on and every moment after it, the same way it hands it
// the notes.

// The ends of the beat somebody can practise to. Under thirty the bar is too
// long to feel and over three hundred the click is a tone.
const (
	MinBpm = 30.0
	MaxBpm = 300.0
)

// ClickOn says whether the metronome is counting.
func (e *Engine) ClickOn() bool { return e.click }

// StartClick sets it going from this moment, which is the only place its clock
// is set. A click that starts halfway through a bar is one nobody can play to.
func (e *Engine) StartClick(now time.Time) {
	e.click = true
	e.clickFrom = now
}

func (e *Engine) StopClick() { e.click = false }

// Bpm is the beat the song is being run at, and the number the screen shows.
// A song carries its own tempo and Speed scales it, so asking for a beat and
// asking for a speed cannot disagree. A text tab carries no tempo, and there
// the number stands on its own and drives the click and nothing else.
func (e *Engine) Bpm() float64 {
	if e.Song.Untimed || e.Song.Tempo <= 0 {
		return e.clickBpm
	}
	return e.Song.Tempo * e.speed()
}

// SetBpm asks for a beat and answers the one it settled on, since the speed it
// is turned into has ends and a song written at 60 cannot be run at 300.
func (e *Engine) SetBpm(bpm float64) float64 {
	if bpm < MinBpm {
		bpm = MinBpm
	}
	if bpm > MaxBpm {
		bpm = MaxBpm
	}

	e.clickBpm = bpm
	if !e.Song.Untimed && e.Song.Tempo > 0 {
		e.SetSpeed(bpm / e.Song.Tempo)
	}

	return e.Bpm()
}

// Click is what the sound side is told: the beat to count and how many of them
// are in the bar. In tempo mode both come off the measure the clock is in,
// since a song may change tempo halfway and a click still counting the tempo
// it opened on would be counting against the notes coming at it.
func (e *Engine) Click(now time.Time) (float64, int) {
	if e.Mode != Tempo || !e.running {
		return e.Bpm(), e.beats(e.Song.MeasureAt(0))
	}

	measure := e.Song.MeasureAt(e.Elapsed(now).Seconds())
	tempo := measure.Tempo
	if tempo <= 0 {
		tempo = e.Song.Tempo
	}

	return tempo * e.speed(), e.beats(measure)
}

// ClickPulse is where the count is inside the bar. It is the song's own beat
// whenever there is one running, so the dots on the screen and the notes
// coming at them are the same clock.
func (e *Engine) ClickPulse(now time.Time) (int, int) {
	if e.Mode == Tempo && e.running {
		return e.Pulse(now)
	}

	bpm, beats := e.Click(now)
	if !e.click || bpm <= 0 {
		return 0, beats
	}

	pulse := int(now.Sub(e.clickFrom).Seconds() * bpm / 60.0)
	if pulse < 0 {
		pulse = 0
	}

	return pulse % beats, beats
}

// beats is how many pulses the bar holds, counted in the unit the signature is
// written in, so six eight is six and not three.
func (e *Engine) beats(measure song.Measure) int {
	if count := measure.Signature[0]; count > 0 {
		return count
	}
	return 4
}
