// Package songsterr looks songs up on songsterr.com.
//
// Only the search is used, and only for what it answers about a song: that a
// tab of it exists, which instruments it was written for and how hard they
// called it. The tab itself is not reachable from here, so what the app does
// with a result is open the page for it.
package songsterr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const endpoint = "https://www.songsterr.com/api/songs"

// agent is sent because the search answers 403 to a bare go client.
const agent = "Mozilla/5.0 (X11; Linux x86_64) fretdeck"

// guitar is the general midi range the guitars sit in, 24 nylon through 31
// harmonics. Bass starts at 32 and is a different instrument to practise.
const (
	guitarFirst = 24
	guitarLast  = 31
)

// Track is one instrument of a transcription.
type Track struct {
	InstrumentID int    `json:"instrumentId"`
	Instrument   string `json:"instrument"`
	Name         string `json:"name"`
	Tuning       []int  `json:"tuning"`
	Difficulty   int    `json:"difficulty"`
	Views        int    `json:"views"`
	Hash         string `json:"hash"`
}

func (t Track) Guitar() bool {
	return t.InstrumentID >= guitarFirst && t.InstrumentID <= guitarLast
}

// Song is one result.
type Song struct {
	SongID int     `json:"songId"`
	Artist string  `json:"artist"`
	Title  string  `json:"title"`
	Tracks []Track `json:"tracks"`
}

// Guitar is the guitar track most people opened, which is the one worth
// reporting a difficulty from. A song written for no guitar answers false.
func (s Song) Guitar() (Track, bool) {
	best, found := Track{}, false
	for _, track := range s.Tracks {
		if !track.Guitar() {
			continue
		}
		if !found || track.Views > best.Views {
			best, found = track, true
		}
	}
	return best, found
}

// Difficulty is what songsterr calls the guitar track, and zero when it says
// nothing. The scale is theirs and runs past five, so it is carried as the
// number it is rather than rounded into stars that would invent a ceiling.
func (s Song) Difficulty() int {
	if track, ok := s.Guitar(); ok {
		return track.Difficulty
	}
	return 0
}

// URL is the page of the song. The slug does not have to be right: songsterr
// redirects to the real one as long as the id at the end is.
func (s Song) URL() string {
	return fmt.Sprintf("https://www.songsterr.com/a/wsa/x-tab-s%d", s.SongID)
}

// Client is the http client used for the search. It exists so the tests can
// point at a server of their own instead of the internet.
type Client struct {
	Base string
	HTTP *http.Client
}

func New() *Client {
	return &Client{Base: endpoint, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// Search asks for songs matching a pattern, guitar songs first.
func (c *Client) Search(ctx context.Context, pattern string) ([]Song, error) {
	address := c.Base + "?pattern=" + url.QueryEscape(pattern)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", agent)
	request.Header.Set("Accept", "application/json")

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("songsterr answered %s", response.Status)
	}

	var songs []Song
	if err := json.NewDecoder(response.Body).Decode(&songs); err != nil {
		return nil, err
	}

	// a song with no guitar in it is not what anybody typed into this app, so
	// it goes to the bottom rather than being dropped: the artist may be right
	sort.SliceStable(songs, func(i, j int) bool {
		_, left := songs[i].Guitar()
		_, right := songs[j].Guitar()
		return left && !right
	})

	return songs, nil
}

// Best is the one result to take for a track that came from somewhere else,
// such as a spotify library. It is the match whose artist and title are the
// same text, ignoring case and punctuation, and nothing at all when none is.
//
// Taking the first result instead would answer a cover, a live version or a
// different song by the same band, and the difficulty it reported would be
// about the wrong transcription.
func Best(songs []Song, artist, title string) (Song, bool) {
	wantArtist, wantTitle := normalize(artist), normalize(title)

	for _, candidate := range songs {
		if normalize(candidate.Artist) != wantArtist {
			continue
		}
		if normalize(candidate.Title) != wantTitle {
			continue
		}
		return candidate, true
	}

	return Song{}, false
}

// normalize strips what the two catalogues disagree about: case, punctuation
// and the remaster note a streaming service puts after a title.
func normalize(text string) string {
	text = strings.ToLower(text)

	for _, cut := range []string{" - ", " (", " ["} {
		if at := strings.Index(text, cut); at > 0 {
			text = text[:at]
		}
	}

	var out strings.Builder
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
		}
	}

	return out.String()
}
