package practice

import (
	"math"
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

type Mode int

const (
	// Wait holds the cursor on a note until it is played right. There is no
	// clock, which is what learning a passage actually looks like
	Wait Mode = iota

	// Tempo runs the song at its own speed and judges every note against the
	// instant it was written for
	Tempo
)

func (m Mode) String() string {
	if m == Wait {
		return "wait"
	}
	return "tempo"
}

type Verdict int

const (
	Pending Verdict = iota
	Hit
	Wrong
	Missed
)

// Result is what happened to one event of the song.
type Result struct {
	Verdict Verdict
	Played  int
	Offset  time.Duration
}

// Score is the line at the bottom of the practice screen.
type Score struct {
	Total     int
	Hits      int
	Wrongs    int
	Missed    int
	Accuracy  float64
	AvgOffset time.Duration
}

// window is how far from its written instant a note still counts in tempo
// mode. A hundred and fifty milliseconds is about where a listener starts to
// hear a note as late rather than as part of the beat.
const window = 150 * time.Millisecond

// countIn is the silence before the clock starts, so nobody is judged on a
// note they were supposed to play before the screen finished drawing.
const countIn = 2 * time.Second

// Engine walks a song one event at a time and judges what it hears.
//
// It owns no audio and no clock of its own: the caller feeds it notes and the
// current time. That is what lets the whole of it be tested without a guitar.
type Engine struct {
	Song   *song.Song
	Events []song.Event
	Mode   Mode

	// Speed multiplies the tempo, so a passage can be run slower than written
	Speed float64

	results  []Result
	cursor   int
	origin   time.Time
	running  bool
	offsets  []time.Duration
	detected int
}

func New(s *song.Song, mode Mode) *Engine {
	events := s.Events()
	return &Engine{
		Song:    s,
		Events:  events,
		Mode:    mode,
		Speed:   1,
		results: make([]Result, len(events)),
	}
}

func (e *Engine) Cursor() int         { return e.cursor }
func (e *Engine) Running() bool       { return e.running }
func (e *Engine) Results() []Result   { return e.results }
func (e *Engine) Detected() int       { return e.detected }
func (e *Engine) Done() bool          { return e.cursor >= len(e.Events) }
func (e *Engine) Result(i int) Result { return e.results[i] }

// Current is the event being waited for, and false once the song is over.
func (e *Engine) Current() (song.Event, bool) {
	if e.Done() {
		return song.Event{}, false
	}
	return e.Events[e.cursor], true
}

func (e *Engine) Start(now time.Time) {
	e.origin = now
	e.running = true
}

func (e *Engine) Stop() {
	e.running = false
}

// Reset puts the song back at the first note and forgets every verdict.
func (e *Engine) Reset() {
	e.results = make([]Result, len(e.Events))
	e.cursor = 0
	e.running = false
	e.offsets = nil
	e.detected = 0
}

// Seek jumps to an event, which is how somebody practises one measure over and
// over without playing the three before it every time.
func (e *Engine) Seek(index int) {
	if index < 0 {
		index = 0
	}
	if index > len(e.Events) {
		index = len(e.Events)
	}
	e.cursor = index
}

// SeekMeasure puts the cursor on the first note of a measure.
func (e *Engine) SeekMeasure(measure int) {
	for index, event := range e.Events {
		if event.Measure >= measure {
			e.Seek(index)
			return
		}
	}
}

// Elapsed is where the clock is inside the song, negative during the count in.
func (e *Engine) Elapsed(now time.Time) time.Duration {
	if !e.running {
		return 0
	}
	speed := e.Speed
	if speed <= 0 {
		speed = 1
	}
	return time.Duration(float64(now.Sub(e.origin)-countIn) * speed)
}

// CountIn is what is left of the count in, and zero once the song is running.
func (e *Engine) CountIn(now time.Time) time.Duration {
	if elapsed := e.Elapsed(now); elapsed < 0 {
		return -elapsed
	}
	return 0
}

// Beat is the position inside the measure, for the flashing metronome. It is
// derived from the tempo of the measure the cursor is in, since a song may
// change tempo halfway through.
func (e *Engine) Beat(now time.Time) float64 {
	elapsed := e.Elapsed(now).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}

	tempo, start, beat := e.Song.Tempo, 0.0, 0.0
	for _, measure := range e.Song.Measures {
		if measure.Time > elapsed {
			break
		}
		tempo, start, beat = measure.Tempo, measure.Time, measure.Beat
	}
	if tempo <= 0 {
		tempo = 120
	}

	return beat + (elapsed-start)*tempo/60.0
}

// Pulse is where the clock is inside the bar: which pulse of the measure, and
// how many the measure holds. It counts in the unit the signature is written
// in, so a bar of six eight is six pulses and not three.
func (e *Engine) Pulse(now time.Time) (int, int) {
	beat := e.Beat(now)

	measure := song.Measure{Beat: 0, Signature: [2]int{4, 4}}
	for _, candidate := range e.Song.Measures {
		if candidate.Beat > beat {
			break
		}
		measure = candidate
	}

	count, unit := measure.Signature[0], measure.Signature[1]
	if count <= 0 {
		count = 4
	}
	if unit <= 0 {
		unit = 4
	}

	// Beat counts in quarter notes, and one pulse of the signature is worth
	// four over its denominator of them
	length := 4.0 / float64(unit)
	pulse := int((beat - measure.Beat) / length)
	if pulse < 0 {
		pulse = 0
	}

	return pulse % count, count
}

// Tick moves the clock in tempo mode and writes off the notes whose moment has
// passed. It does nothing in wait mode, which has no moment to miss.
func (e *Engine) Tick(now time.Time) {
	if !e.running || e.Mode != Tempo {
		return
	}

	elapsed := e.Elapsed(now)
	for e.cursor < len(e.Events) {
		at := time.Duration(e.Events[e.cursor].Time * float64(time.Second))
		if elapsed < at+window {
			return
		}
		if e.results[e.cursor].Verdict == Pending {
			e.results[e.cursor].Verdict = Missed
		}
		e.cursor++
	}
}

// Heard feeds one note from the detector and returns the verdict it produced.
//
// In wait mode a wrong note leaves the cursor where it is, because the point
// of the mode is that the passage does not go on until it is right.
func (e *Engine) Heard(midi int, now time.Time) Verdict {
	if e.Done() {
		return Pending
	}
	e.detected++

	if e.Mode == Wait {
		if !e.Events[e.cursor].Matches(midi) {
			e.results[e.cursor] = Result{Verdict: Wrong, Played: midi}
			return Wrong
		}
		e.results[e.cursor] = Result{Verdict: Hit, Played: midi}
		e.cursor++
		return Hit
	}

	if !e.running || e.Elapsed(now) < 0 {
		return Pending
	}

	elapsed := e.Elapsed(now)
	// the note may belong to the event under the cursor or to the next one,
	// since somebody playing ahead of the beat gets there before Tick does
	for index := e.cursor; index < len(e.Events) && index <= e.cursor+1; index++ {
		at := time.Duration(e.Events[index].Time * float64(time.Second))
		offset := elapsed - at
		if offset < -window || offset > window {
			continue
		}
		if !e.Events[index].Matches(midi) {
			continue
		}
		e.results[index] = Result{Verdict: Hit, Played: midi, Offset: offset}
		e.offsets = append(e.offsets, offset)
		e.cursor = index + 1
		return Hit
	}

	e.results[e.cursor] = Result{Verdict: Wrong, Played: midi}
	return Wrong
}

// Score counts what has been judged so far, not the whole song, so the number
// on the screen means the passage that was actually played.
func (e *Engine) Score() Score {
	score := Score{}
	var total time.Duration

	for index, result := range e.results {
		if index >= e.cursor && result.Verdict == Pending {
			continue
		}
		score.Total++
		switch result.Verdict {
		case Hit:
			score.Hits++
			total += result.Offset
		case Wrong:
			score.Wrongs++
		case Missed:
			score.Missed++
		}
	}

	if score.Total > 0 {
		score.Accuracy = float64(score.Hits) / float64(score.Total)
	}
	if len(e.offsets) > 0 {
		score.AvgOffset = total / time.Duration(len(e.offsets))
	}

	return score
}

// Progress is how far into the song the cursor is, from zero to one.
func (e *Engine) Progress() float64 {
	if len(e.Events) == 0 {
		return 0
	}
	return math.Min(1, float64(e.cursor)/float64(len(e.Events)))
}
