package cifraclub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// exercise is a cifra written here rather than taken off the site, so the test
// owns what it asserts about: a chord over a lyric, which is what most of a
// page there is, and a tab block under it, which is the part that can be
// played. The chord names are in a tag of their own, the way the site draws
// them.
const exercise = "<b>Am</b>\r\ncome as you are\r\n" +
	"E|-------------------|\r\n" +
	"B|-------------------|\r\n" +
	"G|-------------------|\r\n" +
	"D|-------------------|\r\n" +
	"A|-------------0-2-3-|\r\n" +
	"E|-0-2-3-5-7---------|\r\n"

// page is a song page: the cifra in a pre, and the json the app was handed
// with the tuning and the capo escaped inside it.
func page(cifra string) string {
	return `<html><head><title>Some Song - Some Band - Cifra Club</title></head><body>` +
		`<h1 class="t1">Some Song</h1><h2>Menu principal</h2>` +
		`<pre class="a">` + cifra + `</pre>` +
		`<script>self.__next_f.push([1,"{\"artist\":{\"id\":12,\"name\":\"Some Band\"}," +
		"\"config\":{\"capo\":2,\"tuning\":\"Eb Ab Db Gb Bb Eb\"}}"])</script>` +
		`</body></html>`
}

func serve(t *testing.T, body string, status int) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("the request went out with no agent on it")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return &Client{Base: server.URL, Index: server.URL, HTTP: server.Client()}
}

const results = `{"response":{"docs":[
	{"t":"1","art":"Some Band","dns":"some-band","txt":"Some Band"},
	{"t":"2","art":"Some Band","dns":"some-band","txt":"Some Song","url":"some-song"}]}}`

func TestSearchKeepsTheSongsAndDropsTheArtists(t *testing.T) {
	found, err := serve(t, results, http.StatusOK).Search(context.Background(), "some song")
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Fatalf("want the one song, got %d rows", len(found))
	}
	if !strings.HasSuffix(found[0].URL, "/some-band/some-song/") {
		t.Fatalf("the two halves of the address did not go together: %q", found[0].URL)
	}

	// there is one cifra a song there and it is not known what is in it until
	// the page is read, so the row promises nothing
	if found[0].Kind != "" || found[0].Version != 0 {
		t.Fatalf("the row claimed a kind or a version the site does not keep: %+v", found[0])
	}
}

func TestFetchReadsTheCifraAndWhatIsBesideIt(t *testing.T) {
	client := serve(t, page(exercise), http.StatusOK)

	tab, err := client.Fetch(context.Background(), client.Base+"/some-band/some-song/")
	if err != nil {
		t.Fatal(err)
	}

	if tab.Title != "Some Song" || tab.Artist != "Some Band" {
		t.Fatalf("the page was read as %q by %q", tab.Title, tab.Artist)
	}
	if tab.Tuning != "Eb Ab Db Gb Bb Eb" || tab.Capo != 2 {
		t.Fatalf("tuning %q capo %d, and both are beside the text and not in it", tab.Tuning, tab.Capo)
	}
	if strings.Contains(tab.Text, "<b>") || strings.Contains(tab.Text, "\r") {
		t.Fatal("a tag or a carriage return came through, so the cleaner has fallen behind")
	}
	if !strings.Contains(tab.Text, "E|-0-2-3-5-7---------|") {
		t.Fatalf("the tab block did not survive the cleaning:\n%s", tab.Text)
	}
}

func TestFetchSaysSoWhenThePageHasNoCifra(t *testing.T) {
	client := serve(t, "<html><body>nothing here</body></html>", http.StatusOK)

	_, err := client.Fetch(context.Background(), client.Base)
	if err == nil {
		t.Fatal("a page with no pre in it was read as a tab")
	}
}

func TestFetchCarriesTheStatusUp(t *testing.T) {
	client := serve(t, "gone", http.StatusNotFound)

	_, err := client.Fetch(context.Background(), client.Base)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("want the status the site answered, got %v", err)
	}
}
