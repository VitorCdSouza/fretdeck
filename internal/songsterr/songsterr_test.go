package songsterr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// answer is two songs, one written for guitar and one only for piano, which is
// the pair every rule here has to tell apart.
const answer = `[
 {"songId":1,"artist":"Nobody","title":"Only Piano","tracks":[
   {"instrumentId":0,"instrument":"Acoustic Grand","views":10,"difficulty":9,"hash":"piano_a"}]},
 {"songId":2,"artist":"Metallica","title":"Nothing Else Matters","tracks":[
   {"instrumentId":67,"instrument":"Baritone Sax","views":900,"hash":"vocals_a"},
   {"instrumentId":25,"instrument":"Acoustic Guitar (steel)","views":10,"difficulty":2,"hash":"guitar_a"},
   {"instrumentId":27,"instrument":"Electric Guitar (clean)","views":5000,"difficulty":3,"hash":"guitar_b"}]}
]`

func serve(t *testing.T, body string, status int) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pattern") == "" {
			t.Errorf("the search went out with no pattern on it")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return &Client{Base: server.URL, HTTP: server.Client()}
}

func TestSearchPutsTheGuitarSongsFirst(t *testing.T) {
	songs, err := serve(t, answer, http.StatusOK).Search(context.Background(), "metallica")
	if err != nil {
		t.Fatal(err)
	}

	if len(songs) != 2 {
		t.Fatalf("want 2 results, got %d", len(songs))
	}
	if songs[0].Title != "Nothing Else Matters" {
		t.Fatalf("the song with a guitar in it comes first, got %q", songs[0].Title)
	}
}

func TestDifficultyComesFromTheGuitarTrackPeopleOpen(t *testing.T) {
	songs, _ := serve(t, answer, http.StatusOK).Search(context.Background(), "metallica")

	// the sax has no difficulty and the quiet acoustic track says 2. the one
	// that counts is the electric, which is what five thousand people opened
	if got := songs[0].Difficulty(Guitar); got != 3 {
		t.Fatalf("want the difficulty of the popular guitar track, got %d", got)
	}
}

func TestASongWithNoGuitarReportsNoDifficulty(t *testing.T) {
	songs, _ := serve(t, answer, http.StatusOK).Search(context.Background(), "piano")

	piano := songs[1]
	if _, ok := piano.Played(Guitar); ok {
		t.Fatal("a grand piano is not a guitar")
	}
	if got := piano.Difficulty(Guitar); got != 0 {
		t.Fatalf("want no difficulty, got %d", got)
	}
}

func TestAFailedSearchIsAnError(t *testing.T) {
	if _, err := serve(t, "nope", http.StatusForbidden).Search(context.Background(), "x"); err == nil {
		t.Fatal("a 403 has to come back as an error, not as an empty list")
	}
}

func TestURLCarriesTheIDAndSongsterrFixesTheSlug(t *testing.T) {
	song := Song{SongID: 439171, Artist: "Metallica", Title: "Nothing Else Matters"}

	if got := song.URL(); got != "https://www.songsterr.com/a/wsa/x-tab-s439171" {
		t.Fatalf("unexpected url %q", got)
	}
}

func TestBestIgnoresCasePunctuationAndTheRemasterNote(t *testing.T) {
	songs := []Song{
		{SongID: 9, Artist: "Metallica", Title: "Enter Sandman"},
		{SongID: 2, Artist: "Metallica", Title: "Nothing Else Matters"},
	}

	found, ok := Best(songs, "METALLICA", "Nothing Else Matters - Remastered 2021")
	if !ok {
		t.Fatal("the remaster note cannot cost the match")
	}
	if found.SongID != 2 {
		t.Fatalf("matched the wrong song, id %d", found.SongID)
	}
}

func TestBestRefusesADifferentSongByTheSameBand(t *testing.T) {
	songs := []Song{{SongID: 9, Artist: "Metallica", Title: "Enter Sandman"}}

	// taking the first result would report the difficulty of another
	// transcription, which is worse than reporting nothing
	if _, ok := Best(songs, "Metallica", "One"); ok {
		t.Fatal("Enter Sandman is not One")
	}
}

// both is one song transcribed for a guitar and for a bass, which is the pair
// the family has to tell apart.
const both = `[
 {"songId":3,"artist":"Rush","title":"YYZ","tracks":[
   {"instrumentId":30,"instrument":"Distortion Guitar","views":900,"difficulty":6,"hash":"guitar_a"},
   {"instrumentId":33,"instrument":"Electric Bass (finger)","views":400,"difficulty":4,"hash":"bass_a"}]}
]`

// TestTheDifficultyIsTheOneOfTheInstrumentBeingPlayed is why the family is
// carried at all: the same song is two different things to practise.
func TestTheDifficultyIsTheOneOfTheInstrumentBeingPlayed(t *testing.T) {
	songs, err := serve(t, both, http.StatusOK).Search(context.Background(), "yyz")
	if err != nil {
		t.Fatal(err)
	}

	if got := songs[0].Difficulty(Guitar); got != 6 {
		t.Fatalf("want the guitar track, got %d", got)
	}
	if got := songs[0].Difficulty(Bass); got != 4 {
		t.Fatalf("want the bass track, got %d", got)
	}
}

// TestASearchForABassSinksTheSongsWithoutOne is the same sort the guitar gets,
// read for the other instrument.
func TestASearchForABassSinksTheSongsWithoutOne(t *testing.T) {
	client := serve(t, answer, http.StatusOK)
	client.Family = Bass

	songs, err := client.Search(context.Background(), "metallica")
	if err != nil {
		t.Fatal(err)
	}

	// neither has a bass on it, so the order the site answered in is what stays
	if songs[0].Title != "Only Piano" {
		t.Fatalf("a stable sort keeps the answer as it came, got %q first", songs[0].Title)
	}
}
