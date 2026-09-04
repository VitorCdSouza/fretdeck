package cifraclub

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
// cifra is still in the pre and the tuning still in the json, since those are
// the two things the site can move without telling anybody.
func TestLive(t *testing.T) {
	if os.Getenv("FRETDECK_LIVE") == "" {
		t.Skip("set FRETDECK_LIVE=1 to reach cifra club")
	}

	client := New()

	found, err := client.Search(context.Background(), "nirvana come as you are")
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
		fmt.Printf("%-28s %-20s %s\n", item.Title, item.Artist, item.URL)
	}

	tab, err := client.Fetch(context.Background(), found[0].URL)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%q by %q, tuning %q capo %d, %d lines\n",
		tab.Title, tab.Artist, tab.Tuning, tab.Capo, strings.Count(tab.Text, "\n")+1)

	if tab.Title == "" || tab.Artist == "" {
		t.Fatal("the page named neither the song nor who wrote it")
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
