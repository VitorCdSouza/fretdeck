package song

import (
	"testing"
)

// plain is the shape most of the internet writes a tab in: a heading nobody
// asked for, six lines, bar lines, and a two digit fret to catch the reader
// that counts characters instead of numbers.
const plain = `Nothing In Particular
Some Band

Tuning: E A D G B e

e|-----------------|-----------------|
B|-----------------|-------------12--|
G|-------------0---|-----------------|
D|---0---2---------|-----5-----------|
A|-2---------------|-----------------|
E|-----------------|-3---------------|
`

func TestParseASCIIReadsTheNotesInOrder(t *testing.T) {
	parsed, err := ParseASCII(plain, "test")
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		str  int
		fret int
	}{
		{5, 2}, {4, 0}, {4, 2}, {3, 0},
		{6, 3}, {4, 5}, {2, 12},
	}

	if len(parsed.Notes) != len(want) {
		t.Fatalf("want %d notes, got %d", len(want), len(parsed.Notes))
	}
	for index, note := range parsed.Notes {
		if note.String != want[index].str || note.Fret != want[index].fret {
			t.Fatalf("note %d is string %d fret %d, want string %d fret %d",
				index, note.String, note.Fret, want[index].str, want[index].fret)
		}
	}
}

func TestATwoDigitFretIsOneNote(t *testing.T) {
	parsed, _ := ParseASCII(plain, "test")

	last := parsed.Notes[len(parsed.Notes)-1]
	if last.Fret != 12 {
		t.Fatalf("the twelfth fret came out as %d, so the digits were read apart", last.Fret)
	}
	if last.Midi != 59+12 {
		t.Fatalf("the pitch is wrong: %d", last.Midi)
	}
}

func TestBarLinesBecomeMeasures(t *testing.T) {
	parsed, _ := ParseASCII(plain, "test")

	if len(parsed.Measures) != 2 {
		t.Fatalf("want two measures out of the two bars, got %d", len(parsed.Measures))
	}
	if parsed.Notes[0].Measure == parsed.Notes[len(parsed.Notes)-1].Measure {
		t.Fatal("the notes after the bar line belong to the next measure")
	}
}

func TestTheTabIsMarkedUntimed(t *testing.T) {
	parsed, _ := ParseASCII(plain, "test")

	if !parsed.Untimed {
		t.Fatal("a text tab has no rhythm in it and has to say so")
	}
}

func TestLyricsAndChordNamesAreNotTabLines(t *testing.T) {
	// a chord sheet is the other thing on those sites, and the dashes in it
	// are the trap: reading one as a tab would produce a song of nothing
	sheet := `
Em              G
I found a love -- for me

e|-----------------|
B|-------------0---|
G|-----0-----------|
D|-2---------------|
A|-----------------|
E|-----------------|
`

	parsed, err := ParseASCII(sheet, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Notes) != 3 {
		t.Fatalf("want the three notes of the tab block only, got %d", len(parsed.Notes))
	}
}

func TestATechniqueBetweenFretsIsNotAFret(t *testing.T) {
	// h is a hammer on, / is a slide. both join two notes that are both real,
	// and neither is a note of its own
	riff := `
e|--------------|
B|--------------|
G|--------------|
D|--------------|
A|--------------|
E|-5h7-9/12-----|
`

	parsed, _ := ParseASCII(riff, "test")

	var frets []int
	for _, note := range parsed.Notes {
		frets = append(frets, note.Fret)
	}

	want := []int{5, 7, 9, 12}
	if len(frets) != len(want) {
		t.Fatalf("want %v, got %v", want, frets)
	}
	for index := range want {
		if frets[index] != want[index] {
			t.Fatalf("want %v, got %v", want, frets)
		}
	}
}

func TestANamedTuningIsBelieved(t *testing.T) {
	dropped := `
Tuning: D A D G B e

e|--------|
B|--------|
G|--------|
D|--------|
A|--------|
D|-0------|
`

	parsed, err := ParseASCII(dropped, "test")
	if err != nil {
		t.Fatal(err)
	}

	// the sixth string is a D, a tone under the E of standard tuning
	if got := parsed.Tuning[5].Midi; got != 38 {
		t.Fatalf("want the low string at D2, got midi %d", got)
	}
	if parsed.Notes[0].Midi != 38 {
		t.Fatalf("the open sixth string is that D, got midi %d", parsed.Notes[0].Midi)
	}
}

func TestStandardTuningIsTheDefault(t *testing.T) {
	parsed, _ := ParseASCII(plain, "test")

	want := []int{64, 59, 55, 50, 45, 40}
	for index, str := range parsed.Tuning {
		if str.Midi != want[index] {
			t.Fatalf("string %d is midi %d, want %d", str.Number, str.Midi, want[index])
		}
	}
}

func TestAFileWithNoTabInItIsAnError(t *testing.T) {
	if _, err := ParseASCII("just some words about a song", "test"); err == nil {
		t.Fatal("want an error, got a song with nothing in it")
	}
}

func TestAFourStringBlockIsReadAsABass(t *testing.T) {
	bass := `
G|--------|
D|--------|
A|--------|
E|-3------|
`

	parsed, err := ParseASCII(bass, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Tuning) != 4 {
		t.Fatalf("want four strings, got %d", len(parsed.Tuning))
	}
	// a bass low E is an octave under a guitar one
	if parsed.Notes[0].Midi != 28+3 {
		t.Fatalf("want the bass E, got midi %d", parsed.Notes[0].Midi)
	}
}
