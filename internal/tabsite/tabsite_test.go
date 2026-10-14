package tabsite

import "testing"

// TestKeyIgnoresCasePunctuationAndTheRemasterNote is what two rows of a search
// have to agree on to be the one song, and what a streamed track has to agree
// with to be found at all.
func TestKeyIgnoresCasePunctuationAndTheRemasterNote(t *testing.T) {
	want := Key("Metallica", "Nothing Else Matters")

	for _, written := range []string{
		"nothing else matters",
		"Nothing Else Matters - Remastered 2021",
		"Nothing, Else Matters (Live)",
	} {
		if got := Key("METALLICA", written); got != want {
			t.Fatalf("%q keyed as %q, not %q", written, got, want)
		}
	}

	if Key("Metallica", "One") == want {
		t.Fatal("One is not Nothing Else Matters")
	}
}
