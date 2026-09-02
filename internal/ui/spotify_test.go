package ui

import (
	"strings"
	"testing"

	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
)

// TestTheSpotifyScreenIsOneButtonUntilThereIsASession is the whole of the
// screen logged out: there is nothing else that can be done about it.
func TestTheSpotifyScreenIsOneButtonUntilThereIsASession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenSpotify

	drawn := stripAnsi(m.View())
	if !strings.Contains(drawn, "Log in with Spotify") {
		t.Fatal("the login button is the screen while there is no session")
	}
	if m.spotifyRows() != 0 {
		t.Fatalf("there is a list on the screen before the login, %d rows of it", m.spotifyRows())
	}

	// opening the screen asks for nothing while there is no session
	if cmd := m.enterSpotify(); cmd != nil {
		t.Fatal("the library was asked for with nobody logged in")
	}
	if m.stage != stageLogin {
		t.Fatalf("the screen opens on the login, it is on stage %d", m.stage)
	}

	if m.keySpotify("enter") == nil || !m.pulling {
		t.Fatal("enter has to start the login and say it is waiting")
	}
}

// TestAPlaylistIsSortedEasiestFirst is the question somebody brings a playlist
// here to ask. What songsterr could not match sinks to the bottom, since a
// blank means not found and not easy.
func TestAPlaylistIsSortedEasiestFirst(t *testing.T) {
	m := model(t)
	m.linked = true
	m.screen = screenSpotify

	m.showTracks([]spotifyTrack{
		{Artist: "Nobody", Title: "Hard One"},
		{Artist: "Nobody", Title: "Unknown One"},
		{Artist: "Nobody", Title: "Easy One"},
	})

	levels := map[string]int{"Hard One": 7, "Easy One": 2}
	for _, item := range m.tracks {
		level, known := levels[item.Title]
		m.Update(lookupMsg{key: item.Key, level: level, found: known})
	}

	order := []string{}
	for _, item := range m.tracks {
		order = append(order, item.Title)
	}
	want := []string{"Easy One", "Hard One", "Unknown One"}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("the playlist is sorted %v, easiest first is %v", order, want)
		}
	}
}

// TestTheTwoListsAreNotTheSameList is what a screen of its own costs: walking
// from one to the other has to leave both where they were.
func TestTheTwoListsAreNotTheSameList(t *testing.T) {
	m := model(t)
	m.linked = true
	m.results = []finding{{From: sourceUltimate, Title: "a search result"}}
	m.showTracks([]spotifyTrack{{Artist: "Nobody", Title: "a track"}})

	m.screen = screenSpotify
	m.stage = stageTracks
	m.move(1)

	if len(m.results) != 1 || m.results[0].Title != "a search result" {
		t.Fatal("the playlist wrote over what the music screen was showing")
	}
	if m.found != 0 {
		t.Fatalf("the cursor of the music screen moved to %d", m.found)
	}
}

// TestEnterOnATrackSearchesForIt is the way out of the screen: spotify knows
// an artist and a title and no tab at all, so the row is a search.
func TestEnterOnATrackSearchesForIt(t *testing.T) {
	m := model(t)
	m.linked = true
	m.screen = screenSpotify
	m.stage = stageTracks
	m.tracks = []finding{{From: sourceTracks, Artist: "Nobody", Title: "Test Riff"}}

	if cmd := m.keySpotify("enter"); cmd == nil {
		t.Fatal("enter on a track has to go and look for a tab of it")
	}
	if m.screen != screenMusic {
		t.Fatalf("the search happens on the music screen, this one is %s", screenNames[m.screen])
	}
	if m.query != "Nobody Test Riff" {
		t.Fatalf("the search is the artist and the title, it was %q", m.query)
	}

	// l is not enter anywhere in the app, and least of all on a row that goes
	// and reads a page
	m.screen = screenSpotify
	m.stage = stageTracks
	m.query = ""
	m.keySpotify("l")
	if m.query != "" {
		t.Fatal("l went and searched, which is what enter is for")
	}
}

// TestASongOfThePlaylistThatIsHereIsPlayed is the other half of enter: a track
// whose tab was read in already opens instead of being searched for again.
func TestASongOfThePlaylistThatIsHereIsPlayed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.linked = true
	m.songs[0].Path = "/home/somebody/fretdeck/songs/test-riff.json"
	m.screen = screenSpotify
	m.stage = stageTracks
	m.showTracks([]spotifyTrack{{Artist: "Nobody", Title: "Test Riff"}})

	if !m.tracks[0].Have() {
		t.Fatal("the song is in the library, so the row has to say so")
	}

	m.keySpotify("enter")
	if m.screen != screenPractice {
		t.Fatalf("enter on a song that is here opens it, the screen is %s", screenNames[m.screen])
	}
}

// TestTheLoginIsClickable is the one place a click presses instead of
// selecting, because there is nothing on that screen to select.
func TestTheLoginIsClickable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.screen = screenSpotify
	m.View()

	if len(m.clicks) != 1 {
		t.Fatalf("the button is what the screen recorded, %d runs are", len(m.clicks))
	}

	m.mouse(click(6, m.clicks[0].top))
	if !m.pulling {
		t.Fatal("clicking the button did not start the login")
	}
}

// TestChangingTheInstrumentAsksAgain is what the number beside a row means: it
// is the difficulty of the instrument being played, not of the song.
func TestChangingTheInstrumentAsksAgain(t *testing.T) {
	m := model(t)
	m.linked = true
	m.songsterr = songsterr.New()
	m.tracks = []finding{{From: sourceTracks, Artist: "Nobody", Title: "Test Riff",
		Key: "nobody test riff", State: lookupDone, Level: 3}}

	m.screen = screenConfig
	m.configRow = 2
	if cmd := m.keepConfig(); cmd == nil {
		t.Fatal("the playlist has to be looked up again for the new instrument")
	}
	if m.tracks[0].State != lookupWaiting {
		t.Fatalf("the row kept the level of the other instrument, state %d", m.tracks[0].State)
	}
}

// TestTheLibraryIsAskedForOnceARun is what the rate limit costs: spotify holds
// down the client id every librespot login shares, so walking past the screen
// must not spend a request on it.
func TestTheLibraryIsAskedForOnceARun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := model(t)
	m.linked = true

	if cmd := m.goTo(screenSpotify); cmd == nil {
		t.Fatal("opening the screen reads the library the first time")
	}

	m.pulling = false
	m.goTo(screenMusic)

	if cmd := m.goTo(screenSpotify); cmd != nil {
		t.Fatal("walking back onto the screen asked for the library again")
	}

	// r is the retry, and it does not care that it was asked for already
	if cmd := m.keySpotify("r"); cmd == nil {
		t.Fatal("r has to read the library again")
	}
}
