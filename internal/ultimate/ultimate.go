// Package ultimate reads tabs off ultimate-guitar.com.
//
// There is no api. The pages are a javascript app and their html carries no
// tab, but the data the app was handed rides along with it: one div holds the
// whole json store as an html escaped attribute, and the search and the tab
// itself are both read out of that. The scrapers this follows went at the old
// markup, which is gone, and at a headless browser, which is a lot of machinery
// for a string that is already in the page.
package ultimate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const search = "https://www.ultimate-guitar.com/search.php"

// agent is a browser, because the site answers 403 to a bare go client.
const agent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// maxPage is the ceiling on a page read. They run to a couple hundred kilobytes
// and the limit is only so a redirect into something enormous cannot fill
// memory.
const maxPage = 8 << 20

// the names the site gives the things it calls a tab. Only the first two have
// frets in them: chords is a chord sheet, and the other two are binaries behind
// a subscription.
const (
	KindTab   = "Tabs"
	KindBass  = "Bass Tabs"
	KindChord = "Chords"
)

// Result is one row of a search.
type Result struct {
	Artist  string  `json:"artist_name"`
	Title   string  `json:"song_name"`
	Kind    string  `json:"type"`
	Version int     `json:"version"`
	Rating  float64 `json:"rating"`
	Votes   int     `json:"votes"`
	Level   string  `json:"difficulty"`
	URL     string  `json:"tab_url"`
}

// Playable answers whether a kind of transcription has frets in it to read. A
// chord sheet is chord names over lyrics and there is nothing to fret; the pro
// and power files are binaries the site sells and does not serve.
func Playable(kind string) bool {
	return kind == KindTab || kind == KindBass
}

func (r Result) Playable() bool { return Playable(r.Kind) }

// Tab is one transcription, with its text already cleaned of the markers the
// site writes around it.
type Tab struct {
	Artist  string
	Title   string
	Kind    string
	Version int
	Level   string
	Tuning  string
	Capo    int
	Text    string
	URL     string
}

// Client is the http client used here. It exists so the tests can point at a
// server of their own instead of the internet.
type Client struct {
	Base string
	HTTP *http.Client
}

func New() *Client {
	return &Client{Base: search, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// Search asks for every transcription of a song, the ones that can be read
// first.
func (c *Client) Search(ctx context.Context, pattern string) ([]Result, error) {
	address := c.Base + "?search_type=title&value=" + url.QueryEscape(pattern)

	var page struct {
		Results []Result `json:"results"`
	}
	if err := c.load(ctx, address, &page); err != nil {
		return nil, err
	}

	// what cannot be read keeps its place at the bottom rather than being
	// dropped: seeing that a song is only up as chords is an answer too
	sort.SliceStable(page.Results, func(i, j int) bool {
		return page.Results[i].Playable() && !page.Results[j].Playable()
	})

	return page.Results, nil
}

// Fetch reads one tab page. The address is the one a search answered with.
func (c *Client) Fetch(ctx context.Context, address string) (*Tab, error) {
	var page tabPage
	if err := c.load(ctx, address, &page); err != nil {
		return nil, err
	}

	text := Clean(page.View.Wiki.Content)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("that page carries no tab, only a link to buy one")
	}

	tab := &Tab{
		Artist:  page.Tab.Artist,
		Title:   page.Tab.Title,
		Kind:    page.Tab.Kind,
		Version: page.Tab.Version,
		Level:   page.Tab.Level,
		Text:    text,
		URL:     address,
	}

	if meta, ok := page.meta(); ok {
		tab.Tuning = meta.Tuning.Value
		tab.Capo = meta.Capo
		if tab.Level == "" {
			tab.Level = meta.Level
		}
	}

	return tab, nil
}

// markers are what the site writes around the parts of the text: the tab blocks
// and the chord names. They are not the tab, and a line with one left in it
// stops looking like a tab line.
var markers = strings.NewReplacer("[tab]", "", "[/tab]", "", "[ch]", "", "[/ch]", "")

// Clean turns the stored text into the plain tab a parser can read.
func Clean(text string) string {
	return markers.Replace(strings.ReplaceAll(text, "\r\n", "\n"))
}

// tabPage is the part of a tab page's data this reads.
type tabPage struct {
	Tab struct {
		Artist  string `json:"artist_name"`
		Title   string `json:"song_name"`
		Kind    string `json:"type"`
		Version int    `json:"version"`
		Level   string `json:"difficulty"`
	} `json:"tab"`

	View struct {
		Wiki struct {
			Content string `json:"content"`
		} `json:"wiki_tab"`

		// the site writes an empty meta as an array and a full one as an
		// object, so it is decoded on its own and its failure costs only the
		// two fields it carries
		Meta json.RawMessage `json:"meta"`
	} `json:"tab_view"`
}

type tabMeta struct {
	Tuning struct {
		Value string `json:"value"`
	} `json:"tuning"`
	Capo  int    `json:"capo"`
	Level string `json:"difficulty"`
}

func (p tabPage) meta() (tabMeta, bool) {
	var meta tabMeta
	if err := json.Unmarshal(p.View.Meta, &meta); err != nil {
		return tabMeta{}, false
	}
	return meta, true
}

// store is the attribute the whole page is rendered from. Its json is html
// escaped, so it holds no quote of its own and ends at the first one.
var store = regexp.MustCompile(`<div class="js-store" data-content="([^"]*)"`)

// load fetches a page and decodes the store inside it into whatever the caller
// wants out of the page's data.
func (c *Client) load(ctx context.Context, address string, into any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", agent)
	request.Header.Set("Accept", "text/html")

	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ultimate guitar answered %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxPage))
	if err != nil {
		return err
	}

	return decode(body, into)
}

func decode(body []byte, into any) error {
	match := store.FindSubmatch(body)
	if match == nil {
		return errors.New("that page carries no data, so the site has changed shape")
	}

	var wrapper struct {
		Store struct {
			Page struct {
				Data json.RawMessage `json:"data"`
			} `json:"page"`
		} `json:"store"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(string(match[1]))), &wrapper); err != nil {
		return err
	}

	if len(wrapper.Store.Page.Data) == 0 {
		return errors.New("that page carries a store with nothing in it")
	}

	return json.Unmarshal(wrapper.Store.Page.Data, into)
}
