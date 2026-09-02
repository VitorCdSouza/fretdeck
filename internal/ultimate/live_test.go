package ultimate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

// TestLive is not part of the suite: it goes to the internet, which no test in
// this repo may do. It is run by hand with FRETDECK_LIVE=1 to check that the
// store is still where this package looks for it, since that is the one thing
// the site can move without telling anybody.
func TestLive(t *testing.T) {
	if os.Getenv("FRETDECK_LIVE") == "" {
		t.Skip("set FRETDECK_LIVE=1 to reach ultimate guitar")
	}

	client := New()

	found, err := client.Search(context.Background(), "smoke on the water")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("the search answered nothing at all, so the shape has changed")
	}

	for index, item := range found {
		if index > 4 {
			break
		}
		fmt.Printf("%-24s %-18s %-10s v%-3d %.2f of %d\n",
			item.Title, item.Artist, item.Kind, item.Version, item.Rating, item.Votes)
	}

	if !found[0].Playable() {
		t.Fatal("nothing readable came back first, so the type names have changed")
	}

	tab, err := client.Fetch(context.Background(), found[0].URL)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("tuning %q capo %d, %d lines\n",
		tab.Tuning, tab.Capo, strings.Count(tab.Text, "\n")+1)

	if strings.Contains(tab.Text, "[tab]") || strings.Contains(tab.Text, "[ch]") {
		t.Fatal("a marker came through, so the cleaner has fallen behind")
	}

	// the two halves have to fit, or the fetch is answering a shape the parser
	// cannot read and nothing downstream would say so
	parsed, err := song.ParseTuned(tab.Text, tab.Title, tab.Tuning)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%d notes over %d measures, %d strings\n",
		len(parsed.Notes), len(parsed.Measures), len(parsed.Tuning))
}
