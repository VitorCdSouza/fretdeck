package song

import (
	"strings"
	"testing"
)

// bent is the riff with something written on two of its notes, which is what a
// guitar pro file carries and a text tab spells with a letter.
func bent() *Song {
	loaded := riff()
	loaded.Notes[0].Technique = Bend
	loaded.Notes[1].Technique = Hammer
	return loaded
}

func TestTheTabWritesTheLetterBesideTheFret(t *testing.T) {
	loaded := bent()

	view := NewTab(loaded, loaded.Events()).View(0, 60)
	if !strings.Contains(view.Rows[5].At+view.Rows[5].After, "3b") {
		t.Fatalf("the bend is not drawn: %q", view.Rows[5].At+view.Rows[5].After)
	}
}

func TestAMarkReadsBackAsTheTechniqueItSpells(t *testing.T) {
	if got := ReadMark('h'); got != Hammer {
		t.Fatalf("h is a hammer on, got %q", got)
	}
	if got := ReadMark('4'); got != "" {
		t.Fatalf("a fret is not a technique, got %q", got)
	}
	if got := Bend.Mark(); got != "b" {
		t.Fatalf("a bend is written b, got %q", got)
	}
}
