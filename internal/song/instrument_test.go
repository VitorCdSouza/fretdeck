package song

import "testing"

// TestAnInstrumentIsWrittenTheWayATabSiteWritesIt is the string handed to the
// parser and shown on the setup screen: thickest first, which is the opposite
// of how the lines are drawn.
func TestAnInstrumentIsWrittenTheWayATabSiteWritesIt(t *testing.T) {
	for name, want := range map[string]string{
		"guitar":               "E A D G B E",
		"guitar, seven string": "B E A D G B E",
		"bass":                 "E A D G",
		"bass, five string":    "B E A D G",
	} {
		if got := Chosen(name).Written(); got != want {
			t.Fatalf("%s is tuned %q, want %q", name, got, want)
		}
	}
}

// TestAnUnknownInstrumentIsAGuitar is what an older config, or one edited by
// hand, has to fall back to rather than leaving the tuner with no strings.
func TestAnUnknownInstrumentIsAGuitar(t *testing.T) {
	chosen := Chosen("theremin")
	if chosen.Name != "guitar" || len(chosen.Tuning) != 6 {
		t.Fatalf("want the guitar, got %q with %d strings", chosen.Name, len(chosen.Tuning))
	}
	if Chosen("").Name != "guitar" {
		t.Fatal("an unanswered config has to draw a guitar")
	}
}

// TestABassIsToldFromAGuitar is what the search reads to know which half of a
// catalogue answers for what is plugged in.
func TestABassIsToldFromAGuitar(t *testing.T) {
	if Chosen("guitar").Bass {
		t.Fatal("a guitar is not a bass")
	}
	if !Chosen("bass, five string").Bass {
		t.Fatal("a five string bass is a bass")
	}
}
