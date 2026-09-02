package ultimate

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// exercise is a tab written here rather than taken off the site, so the test
// owns what it asserts about. It is a scale up two strings, in the markers the
// site wraps its text in.
const exercise = "[tab]e|-------------------|\r\n" +
	"B|-------------------|\r\n" +
	"G|-------------------|\r\n" +
	"D|-------------------|\r\n" +
	"A|-------------0-2-3-|\r\n" +
	"E|-0-2-3-5-7---------|[/tab]"

// page wraps data the way a page does: one div, the json html escaped inside
// the attribute.
func page(data any) string {
	encoded, err := json.Marshal(map[string]any{
		"store": map[string]any{"page": map[string]any{"data": data}},
	})
	if err != nil {
		panic(err)
	}
	return `<html><body><div class="js-store" data-content="` +
		html.EscapeString(string(encoded)) + `"></div></body></html>`
}

func serve(t *testing.T, body string, status int) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("the request went out with no agent on it, which is a 403")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return &Client{Base: server.URL, HTTP: server.Client()}
}

var results = map[string]any{
	"results": []map[string]any{
		{"song_name": "Some Song", "artist_name": "Some Band", "type": "Chords",
			"version": 1, "rating": 4.9, "votes": 300, "tab_url": "https://example.invalid/chords"},
		{"song_name": "Some Song", "artist_name": "Some Band", "type": "Pro",
			"tab_url": "https://example.invalid/pro"},
		{"song_name": "Some Song", "artist_name": "Some Band", "type": "Tabs",
			"version": 2, "rating": 4.1, "votes": 12, "tab_url": "https://example.invalid/tab"},
	},
}

func TestSearchPutsTheReadableOnesFirst(t *testing.T) {
	found, err := serve(t, page(results), http.StatusOK).Search(context.Background(), "some song")
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 3 {
		t.Fatalf("want 3 rows, got %d", len(found))
	}
	if found[0].Kind != KindTab {
		t.Fatalf("the tab comes first, got %q", found[0].Kind)
	}
	if found[0].Version != 2 || found[0].Votes != 12 {
		t.Fatalf("the row lost its version or its votes: %+v", found[0])
	}
	if found[1].Playable() || found[2].Playable() {
		t.Fatal("a chord sheet and a pro file are not playable and must not say they are")
	}
}

func TestSearchSaysWhenThePageHasNoStore(t *testing.T) {
	_, err := serve(t, "<html>nothing here</html>", http.StatusOK).Search(context.Background(), "x")
	if err == nil {
		t.Fatal("a page with no store is an error, not an empty result")
	}
}

func TestSearchCarriesTheStatusUp(t *testing.T) {
	_, err := serve(t, "", http.StatusForbidden).Search(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want the status in the error, got %v", err)
	}
}

var tabView = map[string]any{
	"tab": map[string]any{
		"song_name": "Some Song", "artist_name": "Some Band",
		"type": "Tabs", "version": 2, "difficulty": "novice",
	},
	"tab_view": map[string]any{
		"wiki_tab": map[string]any{"content": exercise},
		"meta": map[string]any{
			"capo":   2,
			"tuning": map[string]any{"name": "Drop D", "value": "D A D G B E"},
		},
	},
}

func TestFetchReadsTheTabAndItsTuning(t *testing.T) {
	client := serve(t, page(tabView), http.StatusOK)

	tab, err := client.Fetch(context.Background(), client.Base)
	if err != nil {
		t.Fatal(err)
	}

	if tab.Title != "Some Song" || tab.Artist != "Some Band" || tab.Version != 2 {
		t.Fatalf("the tab lost its name: %+v", tab)
	}
	if tab.Tuning != "D A D G B E" || tab.Capo != 2 {
		t.Fatalf("want the tuning and the capo out of meta, got %q and %d", tab.Tuning, tab.Capo)
	}
	if strings.Contains(tab.Text, "[tab]") || strings.Contains(tab.Text, "\r") {
		t.Fatal("the markers and the carriage returns have to be gone before a parser sees the text")
	}
}

// a meta of nothing comes back as an array, which is not the object it is
// elsewhere. losing the tuning is the cost; losing the tab is not.
func TestFetchSurvivesAnEmptyMeta(t *testing.T) {
	view := map[string]any{
		"tab": map[string]any{"song_name": "Some Song", "type": "Tabs"},
		"tab_view": map[string]any{
			"wiki_tab": map[string]any{"content": exercise},
			"meta":     []any{},
		},
	}

	client := serve(t, page(view), http.StatusOK)

	tab, err := client.Fetch(context.Background(), client.Base)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Tuning != "" {
		t.Fatalf("there was no tuning to find, got %q", tab.Tuning)
	}
	if !strings.Contains(tab.Text, "0-2-3-5-7") {
		t.Fatal("the tab itself is what has to come through")
	}
}

func TestFetchRefusesAPageWithNoTabInIt(t *testing.T) {
	view := map[string]any{
		"tab":      map[string]any{"song_name": "Some Song", "type": "Pro"},
		"tab_view": map[string]any{"wiki_tab": map[string]any{"content": ""}},
	}

	client := serve(t, page(view), http.StatusOK)

	if _, err := client.Fetch(context.Background(), client.Base); err == nil {
		t.Fatal("a pro page has nothing to read and saying so is the whole job")
	}
}

func TestCleanLeavesTheChordNameBehindTheMarker(t *testing.T) {
	if got := Clean("[ch]Em[/ch]  [ch]G[/ch]"); got != "Em  G" {
		t.Fatalf("want the names without the markers, got %q", got)
	}
}
