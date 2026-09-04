package song

import (
	"strings"
	"testing"
)

// bent is the riff with something written on two of its notes, which is what a
// guitar pro file carries and a level is asked to take off again.
func bent() *Song {
	loaded := riff()
	loaded.Notes[0].Technique = Bend
	loaded.Notes[1].Technique = Hammer
	return loaded
}

func TestALevelTakesTheTechniqueOffAndLeavesTheNote(t *testing.T) {
	events := bent().EventsAt(Plain)

	if len(events) != 3 {
		t.Fatalf("a level drops no note, want 3 events, got %d", len(events))
	}
	for _, event := range events {
		for _, note := range event.Notes {
			if note.Technique != "" {
				t.Fatalf("plain still asks for %q", note.Technique)
			}
		}
	}
	if got := events[0].Notes[0].Midi; got != 43 {
		t.Fatalf("the note is still the note, got midi %d", got)
	}
}

func TestTheMiddleRungKeepsTheHandsAndDropsTheBend(t *testing.T) {
	events := bent().EventsAt(Basic)

	if got := events[0].Notes[0].Technique; got != "" {
		t.Fatalf("basic is under the bend, got %q", got)
	}
	if got := events[1].Notes[0].Technique; got != Hammer {
		t.Fatalf("basic keeps the hammer on, got %q", got)
	}
}

func TestTheTopRungAsksForEverythingTheFileCarries(t *testing.T) {
	events := bent().EventsAt(Full)

	if got := events[0].Notes[0].Technique; got != Bend {
		t.Fatalf("full asks for the bend, got %q", got)
	}
}

func TestTheTabWritesTheLetterBesideTheFret(t *testing.T) {
	loaded := bent()

	full := NewTab(loaded, loaded.EventsAt(Full)).View(0, 60)
	if !strings.Contains(full.Rows[5].At+full.Rows[5].After, "3b") {
		t.Fatalf("the bend is not drawn: %q", full.Rows[5].At+full.Rows[5].After)
	}

	plain := NewTab(loaded, loaded.EventsAt(Plain)).View(0, 60)
	if strings.Contains(plain.Rows[5].At+plain.Rows[5].After, "3b") {
		t.Fatalf("plain drew a bend it does not ask for: %q", plain.Rows[5].After)
	}
}

func TestHardestIsTheRungTheNoteStandsOn(t *testing.T) {
	if got := Hardest([]Technique{Hammer, Bend, Slide}); got != Bend {
		t.Fatalf("a bend over a hammer on is a bend, got %q", got)
	}
	if got := Hardest(nil); got != "" {
		t.Fatalf("nothing written is nothing to play, got %q", got)
	}
}

func TestALevelReadsBackFromWhatWasKept(t *testing.T) {
	for _, level := range Levels {
		if got := ReadLevel(level.String()); got != level {
			t.Fatalf("%q read back as %q", level, got)
		}
	}
	if got := ReadLevel("whatever was in the file"); got != Plain {
		t.Fatalf("an unknown level is the bottom of the ladder, got %q", got)
	}
}
