// Package cifraclub reads tabs off cifraclub.com.br.
//
// The site is a javascript app and the scrapers written for it drive a
// browser, which is a lot of machinery for a string that is already in the
// page: the cifra is served inside one pre element, tab blocks and all, and
// the tuning and the capo ride along in the json the app was handed. The
// search is the one the site's own field asks, a public solr that answers a
// row for every artist and every song it knows.
package cifraclub

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
	"strconv"
	"strings"
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
)

const (
	// index is the search the site's own field asks
	index = "https://solr.sscdn.co/cifraclub/h/"

	// base is where a page is read from
	base = "https://www.cifraclub.com.br/"
)

// agent is a browser, because a bare go client is not what either host expects
// to be answering.
const agent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// maxPage is the ceiling on a page read. A cifra page is half a megabyte of
// markup around four kilobytes of tab, and the limit is only so a redirect
// into something enormous cannot fill memory.
const maxPage = 8 << 20

// kindSong is what the t field of a search row says a row is. The other kind
// is an artist, which names no page a tab can be read from.
const kindSong = "2"

// Client is the http client used here. It exists so the tests can point at a
// server of their own instead of the internet.
type Client struct {
	Base  string
	Index string
	HTTP  *http.Client
}

func New() *Client {
	return &Client{Base: base, Index: index, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// doc is one row as the search writes it. The names are the site's and they
// are short: art and txt are who wrote it and what it is called, dns and url
// are the two halves of the address of the page.
type doc struct {
	Kind   string `json:"t"`
	Artist string `json:"art"`
	Slug   string `json:"dns"`
	Title  string `json:"txt"`
	Song   string `json:"url"`
}

// Search asks for the songs of a pattern, in the order the site answers them.
//
// A row says nothing about what is on the page it names: there is one cifra a
// song there, with no version and no rating, and whether it has tab blocks in
// it or is only chords over lyrics is not known until it is read. So the kind
// is left empty rather than promised, and the preview is what answers it.
func (c *Client) Search(ctx context.Context, pattern string) ([]tabsite.Result, error) {
	address := strings.TrimSuffix(c.Index, "/") + "/?q=" + url.QueryEscape(pattern)

	var page struct {
		Response struct {
			Docs []doc `json:"docs"`
		} `json:"response"`
	}
	if err := c.load(ctx, address, &page); err != nil {
		return nil, err
	}

	found := make([]tabsite.Result, 0, len(page.Response.Docs))
	for _, item := range page.Response.Docs {
		if item.Kind != kindSong || item.Slug == "" || item.Song == "" {
			continue
		}
		found = append(found, tabsite.Result{
			Artist: item.Artist,
			Title:  item.Title,
			URL:    strings.TrimSuffix(c.Base, "/") + "/" + item.Slug + "/" + item.Song + "/",
		})
	}

	return found, nil
}

// Fetch reads one cifra page. The address is the one a search answered with.
func (c *Client) Fetch(ctx context.Context, address string) (*tabsite.Tab, error) {
	body, err := c.read(ctx, address)
	if err != nil {
		return nil, err
	}

	text := Clean(body)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("that page carries no cifra, so the site has changed shape")
	}

	title, artist := named(body)

	return &tabsite.Tab{
		Artist: artist,
		Title:  title,
		Tuning: catch(tuned, body),
		Capo:   number(catch(capoed, body)),
		Text:   text,
		URL:    address,
	}, nil
}

// blocks is the cifra itself. The whole of it is in one pre on a song page,
// and a page printed for paper breaks it into several, in the order it reads.
var blocks = regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)

// marks are the chord names, which are drawn in a tag of their own inside the
// text. What is wanted is the column the tag stood in, not the tag.
var marks = regexp.MustCompile(`<[^>]*>`)

// Clean takes the cifra out of a page and turns it into the plain text a
// parser can read.
func Clean(body []byte) string {
	var text strings.Builder
	for _, block := range blocks.FindAllSubmatch(body, -1) {
		text.Write(marks.ReplaceAll(block[1], nil))
	}
	return html.UnescapeString(strings.ReplaceAll(text.String(), "\r\n", "\n"))
}

// what the page says about itself. The heading is the song and the name of the
// artist is in the json the app was handed, escaped inside it, which is also
// where the tuning and the capo are. The title of the document is the fallback
// for both, since it carries the two of them with the site's own name after.
var (
	heading = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	titled  = regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	credit  = regexp.MustCompile(`\\"artist\\":\{\\"id\\":\d+,\\"name\\":\\"([^\\"]*)`)
	tuned   = regexp.MustCompile(`\\"tuning\\":\\"([^\\"]*)`)
	capoed  = regexp.MustCompile(`\\"capo\\":(\d+)`)
)

// named is the song and who wrote it.
func named(body []byte) (title, artist string) {
	title, artist = plain(catch(heading, body)), plain(catch(credit, body))

	// the document title is the two of them and the site, apart by a dash
	parts := strings.Split(plain(catch(titled, body)), " - ")
	if title == "" && len(parts) > 0 {
		title = parts[0]
	}
	if artist == "" && len(parts) > 1 {
		artist = parts[1]
	}

	return title, artist
}

func catch(pattern *regexp.Regexp, body []byte) string {
	match := pattern.FindSubmatch(body)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func plain(text string) string {
	return strings.TrimSpace(html.UnescapeString(string(marks.ReplaceAllString(text, ""))))
}

func number(text string) int {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return value
}

// load fetches a page and decodes the json it is made of.
func (c *Client) load(ctx context.Context, address string, into any) error {
	body, err := c.read(ctx, address)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

func (c *Client) read(ctx context.Context, address string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", agent)

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cifra club answered %s", response.Status)
	}

	return io.ReadAll(io.LimitReader(response.Body, maxPage))
}
