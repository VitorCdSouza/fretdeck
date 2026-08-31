package song

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// String is one course of the instrument. Number 1 is the thinnest, which is
// how guitar pro counts them and how a tab is drawn, top line first.
type String struct {
	Number int `json:"string"`
	Midi   int `json:"midi"`
}

// Measure carries what the bar line needs to draw itself and the tempo that
// holds from it on, since a song may change tempo halfway.
type Measure struct {
	Index     int     `json:"index"`
	Beat      float64 `json:"beat"`
	Time      float64 `json:"time"`
	Tempo     float64 `json:"tempo"`
	Signature [2]int  `json:"signature"`
}

// Note is one fretted note. Time is in seconds from the start of the song and
// Beat is the same instant in quarter notes, because the tab is drawn in beats
// and the clock runs in seconds.
type Note struct {
	Measure int     `json:"measure"`
	Beat    float64 `json:"beat"`
	Time    float64 `json:"time"`
	Dur     float64 `json:"dur"`
	String  int     `json:"string"`
	Fret    int     `json:"fret"`
	Midi    int     `json:"midi"`
}

type Song struct {
	Title    string    `json:"title"`
	Artist   string    `json:"artist"`
	Track    string    `json:"track"`
	Tempo    float64   `json:"tempo"`
	Tuning   []String  `json:"tuning"`
	Measures []Measure `json:"measures"`
	Notes    []Note    `json:"notes"`

	// Path is where the file was read from. It is not part of the format
	Path string `json:"-"`
}

// Event is everything struck at the same instant. A chord is one event, and
// the practice engine and the tab both count in events, never in notes: six
// strings hit by one strum are one thing to play and one column to draw.
type Event struct {
	Time    float64
	Beat    float64
	Measure int
	Notes   []Note
}

// Midis is what would satisfy this event, in the order the strings ring.
func (e Event) Midis() []int {
	out := make([]int, len(e.Notes))
	for i, note := range e.Notes {
		out[i] = note.Midi
	}
	return out
}

// Matches answers whether a heard pitch counts as this event being played.
//
// An octave off counts. The fundamental of a wound string is quieter than its
// second harmonic and the detector follows the harmonic often enough that
// refusing it would fail people for playing the right note.
func (e Event) Matches(midi int) bool {
	for _, note := range e.Notes {
		if note.Midi == midi || note.Midi-midi == 12 || midi-note.Midi == 12 {
			return true
		}
	}
	return false
}

// Wound answers whether the string is one of the wrapped ones, which the tab
// draws with a heavier line. It is the bottom half of the set on any
// instrument, which is right for a guitar and close enough for the rest.
func (s *Song) Wound(number int) bool {
	return float64(number) > float64(len(s.Tuning))/2
}

// Label names the string for a neck drawn from another song's tuning.
func (s *String) Label(owner *Song) string {
	return owner.Label(s.Number)
}

// Label is the letter shown at the left of each line of the tab.
func (s *Song) Label(number int) string {
	for _, str := range s.Tuning {
		if str.Number != number {
			continue
		}
		letter := strings.TrimRight(NoteName(str.Midi), "0123456789")
		// only the top string is written lowercase, which is how a tab tells
		// the high e from the low E without repeating itself
		if number == 1 {
			return strings.ToLower(letter)
		}
		return letter
	}
	return "?"
}

// Events groups the notes by attack. Notes closer than a thirty second note at
// 200 bpm are the same strum, and no tab writes two columns for that.
func (s *Song) Events() []Event {
	var events []Event

	for _, note := range s.Notes {
		if n := len(events); n > 0 && math.Abs(note.Time-events[n-1].Time) < 0.02 {
			events[n-1].Notes = append(events[n-1].Notes, note)
			continue
		}
		events = append(events, Event{
			Time:    note.Time,
			Beat:    note.Beat,
			Measure: note.Measure,
			Notes:   []Note{note},
		})
	}

	return events
}

// BeatAt turns a moment into a position in quarter notes.
//
// It is not one multiplication: a song may change tempo, so the answer is read
// off the measure the moment falls in and counted from there.
func (s *Song) BeatAt(seconds float64) float64 {
	tempo, start, beat := s.Tempo, 0.0, 0.0
	for _, measure := range s.Measures {
		if measure.Time > seconds {
			break
		}
		tempo, start, beat = measure.Tempo, measure.Time, measure.Beat
	}

	if tempo <= 0 {
		tempo = 120
	}

	return beat + (seconds-start)*tempo/60.0
}

// MeasureAt is the measure a moment falls in, and the beat that measure starts
// on, which is what a bar line needs to draw itself.
func (s *Song) MeasureAt(seconds float64) Measure {
	found := Measure{Index: 1, Signature: [2]int{4, 4}, Tempo: s.Tempo}
	for _, measure := range s.Measures {
		if measure.Time > seconds {
			break
		}
		found = measure
	}
	return found
}

// Duration is where the last note stops ringing.
func (s *Song) Duration() float64 {
	var end float64
	for _, note := range s.Notes {
		if stop := note.Time + note.Dur; stop > end {
			end = stop
		}
	}
	return end
}

func Load(path string) (*Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var parsed Song
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	if len(parsed.Tuning) == 0 {
		return nil, errors.New("the song has no tuning, so no tab can be drawn")
	}

	sort.Slice(parsed.Tuning, func(i, j int) bool {
		return parsed.Tuning[i].Number < parsed.Tuning[j].Number
	})
	sort.SliceStable(parsed.Notes, func(i, j int) bool {
		return parsed.Notes[i].Time < parsed.Notes[j].Time
	})

	parsed.Path = path
	return &parsed, nil
}

// List reads every song in the library folder. A file that does not parse is
// skipped rather than fatal: one bad import should not empty the screen.
func List(dir string) ([]*Song, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var songs []*Song
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		loaded, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		songs = append(songs, loaded)
	}

	sort.Slice(songs, func(i, j int) bool {
		return strings.ToLower(songs[i].Title) < strings.ToLower(songs[j].Title)
	})

	return songs, nil
}

var noteNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// NoteName turns a midi number into the name a tuner shows, C4 being middle C.
func NoteName(midi int) string {
	return fmt.Sprintf("%s%d", noteNames[((midi%12)+12)%12], midi/12-1)
}
