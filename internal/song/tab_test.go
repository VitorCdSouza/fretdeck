package song

import (
	"strings"
	"testing"
)

// riff is two measures of eighth notes plus a chord, which is enough to hold
// every rule the tab has: bar lines, note spacing and a column with more than
// one string in it.
func riff() *Song {
	return &Song{
		Title:  "riff",
		Tempo:  120,
		Tuning: []String{{1, 64}, {2, 59}, {3, 55}, {4, 50}, {5, 45}, {6, 40}},
		Measures: []Measure{
			{Index: 1, Beat: 0, Time: 0, Tempo: 120, Signature: [2]int{4, 4}},
			{Index: 2, Beat: 4, Time: 2, Tempo: 120, Signature: [2]int{4, 4}},
		},
		Notes: []Note{
			{Measure: 1, Beat: 0, Time: 0, Dur: 0.5, String: 6, Fret: 3, Midi: 43},
			{Measure: 1, Beat: 1, Time: 0.5, Dur: 0.5, String: 5, Fret: 12, Midi: 57},
			{Measure: 2, Beat: 4, Time: 2, Dur: 1, String: 3, Fret: 0, Midi: 55},
			{Measure: 2, Beat: 4, Time: 2, Dur: 1, String: 2, Fret: 1, Midi: 60},
		},
	}
}

func TestEventsGroupsAChordIntoOneColumn(t *testing.T) {
	events := riff().Events()

	if len(events) != 3 {
		t.Fatalf("want 3 events, got %d", len(events))
	}
	if len(events[2].Notes) != 2 {
		t.Fatalf("want the chord as one event of 2 notes, got %d", len(events[2].Notes))
	}
}

func TestMatchesAcceptsTheOctaveTheDetectorReports(t *testing.T) {
	event := riff().Events()[0]

	if !event.Matches(43) {
		t.Fatal("the written note has to match itself")
	}
	if !event.Matches(55) {
		t.Fatal("an octave up is the harmonic the detector follows and has to match")
	}
	if event.Matches(44) {
		t.Fatal("a semitone off is a wrong note")
	}
}

func TestLabelsFollowTabConvention(t *testing.T) {
	s := riff()

	if got := s.Label(1); got != "e" {
		t.Fatalf("the top string is written lowercase, got %q", got)
	}
	if got := s.Label(6); got != "E" {
		t.Fatalf("the low string is written uppercase, got %q", got)
	}
}

func TestWoundStringsAreTheBottomHalf(t *testing.T) {
	s := riff()

	if s.Wound(3) {
		t.Fatal("the G string is plain")
	}
	if !s.Wound(4) {
		t.Fatal("the D string is wound")
	}
}

func TestViewDrawsTheBarAndTheFrets(t *testing.T) {
	s := riff()
	events := s.Events()
	view := NewTab(s, events).View(0, 60)

	line := view.Rows[5].Before + view.Rows[5].At + view.Rows[5].After
	if !strings.Contains(line, "3") {
		t.Fatalf("the low E line lost its fret: %q", line)
	}
	if !strings.Contains(line, "━") {
		t.Fatalf("a wound string is drawn with the heavy line: %q", line)
	}
	if strings.Contains(view.Rows[0].After, "━") {
		t.Fatalf("a plain string is drawn with the light line: %q", view.Rows[0].After)
	}
	if !strings.Contains(view.Header, "2") {
		t.Fatalf("the second measure has to be numbered: %q", view.Header)
	}
}

func TestViewKeepsEveryRowTheSameWidth(t *testing.T) {
	s := riff()
	view := NewTab(s, s.Events()).View(1, 40)

	width := len([]rune(view.Rows[0].Before + view.Rows[0].At + view.Rows[0].After))
	for index, row := range view.Rows {
		got := len([]rune(row.Before + row.At + row.After))
		if got != width {
			t.Fatalf("row %d is %d cells wide, the first is %d", index, got, width)
		}
	}
}

func TestATwoDigitFretStillFitsItsColumn(t *testing.T) {
	s := riff()
	view := NewTab(s, s.Events()).View(1, 60)

	if !strings.Contains(view.Rows[4].At, "12") {
		t.Fatalf("the twelfth fret was cut: %q", view.Rows[4].At)
	}
}

// The riff ends on a chord of the G and B strings, which is the case that
// says whether the cursor column marks the strings it plays or the whole
// column.
func TestTheCursorMarksOnlyTheStringsItPlays(t *testing.T) {
	s := riff()
	view := NewTab(s, s.Events()).View(2, 60)

	if view.Rows[1].At != "1" || view.Rows[2].At != "0" {
		t.Fatalf("the chord lost a string: B is %q and G is %q", view.Rows[1].At, view.Rows[2].At)
	}
	for _, index := range []int{0, 3, 4, 5} {
		if view.Rows[index].At != "" {
			t.Fatalf("row %d is not in the chord and was marked as %q", index, view.Rows[index].At)
		}
	}
	if strings.ContainsAny(view.Rows[1].At, "─━") {
		t.Fatalf("the line beside the fret is not the cursor: %q", view.Rows[1].At)
	}
}
