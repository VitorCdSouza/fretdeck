package scripts

import "testing"

// TestOnlyTheRealScriptsAreEmbedded is the guard on the embed line: a glob
// there would carry the tests into the binary.
func TestOnlyTheRealScriptsAreEmbedded(t *testing.T) {
	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"pitch.py": true, "worker.py": true, "gpimport.py": true,
		"spotify.py": true,
	}
	if len(entries) != len(want) {
		t.Fatalf("want %d scripts embedded, got %d", len(want), len(entries))
	}
	for _, entry := range entries {
		if !want[entry.Name()] {
			t.Fatalf("%s has no business inside the binary", entry.Name())
		}
	}
}
