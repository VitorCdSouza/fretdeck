// Package tabsite is the shape every tab site answers in.
//
// Two of them are read here and they are not alike: one has a dozen rated
// versions of a song and the other has one of it and no rating at all. What
// they have in common is a row of a list and a transcription behind it, so
// that is what lives here, and the screens know nothing about where a song
// came from beyond what these two carry.
package tabsite

import "context"

// the names the sites give the things they call a tab. Only the first two have
// frets in them: chords is a chord sheet, and what a site sells is a binary it
// does not serve.
const (
	KindTab   = "Tabs"
	KindBass  = "Bass Tabs"
	KindChord = "Chords"
)

// the names the config keeps a site by. They are written the way the config
// screen says them, since a config is read by people too.
const (
	Ultimate = "ultimate guitar"
	Cifra    = "cifra club"
)

// Info is one site as the config screen lists it, and Sites is that list. Tag
// is the two letters a song read off it is marked with, since the name is
// wider than the column a played song is drawn in.
type Info struct {
	Name string
	Tag  string
	Note string
}

var Sites = []Info{
	{Ultimate, "ug", "a dozen rated versions of a song, in english"},
	{Cifra, "cc", "one cifra a song, brazilian, with the tab blocks inside it"},
}

// TagOf is the mark of a site by name, and nothing for a song that came off no
// site: one imported from a file is still a song on the list.
func TagOf(name string) string {
	for _, site := range Sites {
		if site.Name == name {
			return site.Tag
		}
	}
	return ""
}

// Known answers whether a name is one of the sites. A config is a file and it
// can carry anything, so what it says is checked before it is opened.
func Known(name string) bool {
	for _, site := range Sites {
		if site.Name == name {
			return true
		}
	}
	return false
}

// Site is a place a song is read off. Search answers the rows of a list and
// Fetch reads the page one of them names, which is the whole of what a screen
// here asks of a site.
type Site interface {
	Search(ctx context.Context, pattern string) ([]Result, error)
	Fetch(ctx context.Context, address string) (*Tab, error)
}

// Result is one row of a search. A site that keeps no rating and no version of
// its own answers zero for the three numbers, and the list says nothing about
// them rather than printing a nought.
type Result struct {
	Artist  string
	Title   string
	Kind    string
	Version int
	Rating  float64
	Votes   int
	Level   string
	URL     string
}

// Playable answers whether a kind of transcription has frets in it to read. A
// chord sheet is chord names over lyrics and there is nothing to fret.
func Playable(kind string) bool {
	return kind == KindTab || kind == KindBass
}

func (r Result) Playable() bool { return Playable(r.Kind) }

// Tab is one transcription, with its text already cleaned of whatever the site
// writes around it. The tuning is kept beside the text on both sites, not in
// it, which is why it is carried here and handed to the parser.
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
