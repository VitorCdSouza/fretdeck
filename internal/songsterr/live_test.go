package songsterr

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestLive is not part of the suite: it goes to the internet, which no test in
// this repo may do. It is run by hand with FRETDECK_LIVE=1 to check that the
// search still answers the shape this package expects.
func TestLive(t *testing.T) {
	if os.Getenv("FRETDECK_LIVE") == "" {
		t.Skip("set FRETDECK_LIVE=1 to reach songsterr")
	}

	songs, err := New().Search(context.Background(), "nothing else matters")
	if err != nil {
		t.Fatal(err)
	}

	for index, item := range songs {
		if index > 4 {
			break
		}
		track, ok := item.Played(Guitar)
		fmt.Printf("%-34s %-22s level %d  guitar %v %s\n",
			item.Title, item.Artist, item.Difficulty(Guitar), ok, track.Instrument)
	}

	best, ok := Best(songs, "Metallica", "Nothing Else Matters - Remastered")
	fmt.Println("best:", best.Title, best.URL(), "matched:", ok)
}
