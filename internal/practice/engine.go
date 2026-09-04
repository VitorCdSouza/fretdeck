package practice

import (
	"sort"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

type Verdict int

const (
	Pending Verdict = iota
	Hit
	Wrong
)

// Result is what happened to one event of the song.
type Result struct {
	Verdict Verdict
	Played  int
}

// Engine walks a song one event at a time and judges what it hears.
//
// It owns no audio and no clock: the caller hands it the notes. The cursor
// holds on an event until it is played right, which is what learning a passage
// actually looks like, and it is the keys that move it anywhere else.
type Engine struct {
	Song   *song.Song
	Events []song.Event

	results []Result
	cursor  int

	// repeat is the measures that were picked to be looped over. A song with
	// none of them runs to the end, which is every song until somebody asks
	repeat map[int]bool
	passes int
}

func New(s *song.Song) *Engine {
	events := s.Events()
	return &Engine{
		Song:    s,
		Events:  events,
		results: make([]Result, len(events)),
	}
}

func (e *Engine) Cursor() int         { return e.cursor }
func (e *Engine) Passes() int         { return e.passes }
func (e *Engine) Results() []Result   { return e.results }
func (e *Engine) Result(i int) Result { return e.results[i] }

// Empty is a song with nothing in it to play, which is the one state with no
// event under the cursor. The cursor never runs off the end of a song that has
// notes in it: the last note is where it stops.
func (e *Engine) Empty() bool { return e.cursor >= len(e.Events) }

// Current is the event being waited for.
func (e *Engine) Current() (song.Event, bool) {
	if e.Empty() {
		return song.Event{}, false
	}
	return e.Events[e.cursor], true
}

// Seek jumps to an event, which is how somebody practises one measure over and
// over without playing the three before it every time. The last note is as far
// as it goes: a cursor past the end is a screen whose caret and whose measure
// number are about two different places.
func (e *Engine) Seek(index int) {
	if index < 0 {
		index = 0
	}
	if last := len(e.Events) - 1; index > last {
		index = last
	}
	if index < 0 {
		index = 0
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

// ToggleRepeat picks a measure to be looped over, or drops it. A measure is
// the unit because it is the one a passage is named by, and it is what the
// screen and the keys already move through.
func (e *Engine) ToggleRepeat(measure int) {
	if e.repeat == nil {
		e.repeat = map[int]bool{}
	}
	if e.repeat[measure] {
		delete(e.repeat, measure)
		return
	}
	e.repeat[measure] = true
}

// Repeats says whether a measure is one of the picked ones, which is what the
// tab is marked from.
func (e *Engine) Repeats(measure int) bool { return e.repeat[measure] }

// Looping says there is a passage to come back to.
func (e *Engine) Looping() bool { return len(e.repeat) > 0 }

// Measures is the picked measures in the order they are played.
func (e *Engine) Measures() []int {
	picked := make([]int, 0, len(e.repeat))
	for measure := range e.repeat {
		picked = append(picked, measure)
	}
	sort.Ints(picked)
	return picked
}

// ClearRepeat drops the lot and the song runs to its end again.
func (e *Engine) ClearRepeat() {
	e.repeat = nil
	e.passes = 0
}

// Loop puts the cursor back at the top of the picked measures whenever it is
// outside them, and says whether it had to. Heard calls it as the cursor moves
// on, and the screen calls it when a measure is picked, since a loop nobody is
// inside of repeats nothing.
func (e *Engine) Loop() bool {
	if !e.Looping() {
		return false
	}
	if !e.Empty() && e.repeat[e.Events[e.cursor].Measure] {
		return false
	}

	first := e.repeatStart()
	if first < 0 {
		return false
	}

	// the pass that just ended is not this one, so what it was judged on goes
	// with it and what the screen paints is about the take being played now
	for index := range e.Events {
		if e.repeat[e.Events[index].Measure] {
			e.results[index] = Result{}
		}
	}

	if e.cursor != first {
		e.passes++
	}
	e.cursor = first

	return true
}

// repeatStart is the first event of the earliest picked measure.
func (e *Engine) repeatStart() int {
	for index, event := range e.Events {
		if e.repeat[event.Measure] {
			return index
		}
	}
	return -1
}

// Heard feeds one note from the detector and returns the verdict it produced.
// A wrong note leaves the cursor where it is, because the point of the whole
// thing is that the passage does not go on until it is right.
func (e *Engine) Heard(midi int) Verdict {
	if e.Empty() {
		return Pending
	}

	if !e.Events[e.cursor].Matches(midi) {
		e.results[e.cursor] = Result{Verdict: Wrong, Played: midi}
		return Wrong
	}

	e.results[e.cursor] = Result{Verdict: Hit, Played: midi}

	// the step past the last note is what a passage ending on it comes round
	// from, and Seek is what keeps the cursor inside the song after it
	e.cursor++
	e.Loop()
	e.Seek(e.cursor)

	return Hit
}
