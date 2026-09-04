package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/cifraclub"
	"github.com/VitorCdSouza/fretdeck/internal/config"
	"github.com/VitorCdSouza/fretdeck/internal/song"
	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
	"github.com/VitorCdSouza/fretdeck/internal/ultimate"
)

// openSite is the one place a site is built, so a name out of the config is
// the only thing the rest of the app knows about where a song comes from. A
// name nobody recognises reads ultimate guitar, which is what a config with
// nothing in it means.
func openSite(name string) tabsite.Site {
	if name == tabsite.Cifra {
		return cifraclub.New()
	}
	return ultimate.New()
}

// siteOf is the site a page is read by, which is the one it came from and not
// whichever the config names now: the recent list keeps addresses from before
// the site was last changed, and neither site can read the other's page. What
// is not one of theirs is read by the site in force, since that is where it
// was found.
func siteOf(address string, chosen tabsite.Site) tabsite.Site {
	switch siteName(address) {
	case tabsite.Cifra:
		return cifraclub.New()
	case tabsite.Ultimate:
		return ultimate.New()
	}
	return chosen
}

// siteName is the site an address belongs to, by the host in it. A song with
// no page of its own belongs to none, which is what an empty name says.
func siteName(address string) string {
	switch {
	case strings.Contains(address, "cifraclub."):
		return tabsite.Cifra
	case strings.Contains(address, "ultimate-guitar."):
		return tabsite.Ultimate
	}
	return ""
}

// The one way in. Every song comes through here: the search reads tabs off
// whichever site the config names, and beside it, down the left, is what has
// been played,
// which is the way back to whatever was being worked on.
//
// The two are columns of the one screen and both are drawn the whole time, so
// coming back to a song costs no keystroke. h and l walk between them.
//
// Songsterr is still here and is no longer a screen. What it answers is the
// difficulty beside a row, looked up quietly behind the list.

// source says where a row came from. The three produce the same kind of row,
// which is why they are drawn by the one function: a song, what a site says
// about it, and whether it is already in the library. Two screens draw them,
// so it is carried on the row itself and not on the model.
type source int

const (
	sourceRecent source = iota
	sourceSearch
	sourceTracks
)

// pane is which of the two columns of the music screen the keys are on. What
// was played is the one it opens on, since a search nobody has typed yet has
// nothing to walk through.
type pane int

const (
	paneRecent pane = iota
	paneSearch
)

// keptLines is how many lines a song of the left column takes. The title, and
// under it who wrote it and when it was last played: one line cannot hold the
// three in a column that narrow, and the title on its own is not a song list.
const keptLines = 2

// finding is one row.
type finding struct {
	From   source
	Artist string
	Title  string

	// Key is the artist and the title normalized, which is what a difficulty
	// is looked up and remembered by
	Key   string
	Level int
	State lookup

	// URL is the page the tab is read from, and Path is where it was written
	// when it is on disk. A row can have either, both or neither
	URL  string
	Path string

	// Kind, Version, Rating and Votes are what a rated site's row carries.
	// Played is what a row of the recent list carries instead
	Kind    string
	Version int
	Rating  float64
	Votes   int
	Played  time.Time

	// Group is the song a version belongs to, and Count is how many versions
	// are in it. A row with a Count is the head of its group and the best of
	// them; one with a Count of one is that version and nothing to open
	Group string
	Count int
	Open  bool
}

// heads answers whether a row opens into others, which is the one thing that
// makes enter mean something else on it.
func (f finding) heads() bool { return f.Count > 1 }

// under answers whether a row is one of the versions of an opened song rather
// than a song on the list. Its name is on the row above it, so it is drawn as
// the version it is.
func (f finding) under() bool { return f.Group != "" && f.Count == 0 }

func (f finding) Have() bool { return f.Path != "" }

// entry is the row as the recent list keeps it.
func (f finding) entry() config.Entry {
	return config.Entry{
		Artist:  f.Artist,
		Title:   f.Title,
		Version: f.Version,
		URL:     f.URL,
		File:    f.Path,
	}
}

type lookup int

const (
	lookupDone lookup = iota
	lookupWaiting
	lookupMissing
)

// searchMsg is a search, and grabMsg is one of its tabs read and written
// into the library, which is the whole of that trip.
type searchMsg struct {
	results []tabsite.Result
	err     error
}

type grabMsg struct {
	title    string
	url      string
	path     string
	artist   string
	version  int
	notes    int
	measures int
	err      error
}

// lookupMsg is what songsterr says about one song, and it answers for every
// row of that song.
type lookupMsg struct {
	key   string
	level int
	found bool
}

func (m *Model) searchSite(pattern string) tea.Cmd {
	client := m.site
	return func() tea.Msg {
		results, err := client.Search(context.Background(), pattern)
		return searchMsg{results: results, err: err}
	}
}

// showTabs turns what a site answered into rows: one per song, with the
// versions of it held aside until the row is opened. A search there answers a
// dozen transcriptions of the one song and a list of twenty rows for three
// songs is a list nobody reads. What cannot be read stays on the list rather
// than disappearing, since knowing a song is only up as chords is worth the
// line it takes.
func (m *Model) showTabs(results []tabsite.Result) {
	m.results = nil
	m.groups = map[string][]finding{}

	// the order the site answered in is the relevance of what was typed, so
	// the groups keep it and only the versions inside one are reordered
	var order []string
	for _, item := range results {
		row := finding{
			From:    sourceSearch,
			Artist:  item.Artist,
			Title:   item.Title,
			Key:     songsterr.Key(item.Artist, item.Title),
			State:   lookupWaiting,
			Kind:    item.Kind,
			Version: item.Version,
			Rating:  item.Rating,
			Votes:   item.Votes,
			URL:     item.URL,
			Path:    m.owned(item.Artist, item.Title),
			Group:   groupOf(item),
		}

		if _, held := m.groups[row.Group]; !held {
			order = append(order, row.Group)
		}
		m.groups[row.Group] = append(m.groups[row.Group], row)
	}

	for _, key := range order {
		versions := m.groups[key]
		sort.SliceStable(versions, func(i, j int) bool {
			return popularity(versions[i]) > popularity(versions[j])
		})
		m.groups[key] = versions

		// the row on the list is the best version of the group, so the preview
		// reads that one and a group of one needs no opening at all
		head := versions[0]
		head.Count = len(versions)
		m.results = append(m.results, head)
	}

	m.sortByInstrument()
	m.found = 0
	m.focus = paneSearch
}

// groupOf is what tells one song from another on that list. The instrument is
// part of it: a bass transcription is not a version of the guitar one, it is
// somebody else's part, and a chord sheet is neither.
func groupOf(item tabsite.Result) string {
	return songsterr.Key(item.Artist, item.Title) + " " + item.Kind
}

// prior and middling are what a rating is weighed against, so the versions of
// a song come out in the order people actually rate them. A 5.0 that two
// people voted on is not the best transcription of anything and a plain sort
// by rating puts it over the one three hundred people stand behind.
const (
	prior    = 25.0
	middling = 3.0
)

// popularity is the rating a version is ordered by, pulled towards middling by
// how few votes it has. An unrated one lands on middling itself, which is
// under anything well liked and over anything people voted down.
func popularity(item finding) float64 {
	votes := float64(item.Votes)
	return (votes*item.Rating + prior*middling) / (votes + prior)
}

// expand puts the versions of a group under its row, the best liked first, and
// takes them away again. What songsterr said is the song's and not the
// version's, so it is carried down rather than looked up a second time.
func (m *Model) expand(index int) {
	head := m.results[index]
	if head.Open {
		m.collapse(index)
		return
	}

	rows := make([]finding, 0, head.Count)
	for _, version := range m.groups[head.Group] {
		version.State, version.Level = head.State, head.Level
		rows = append(rows, version)
	}

	m.results[index].Open = true
	m.results = append(m.results[:index+1], append(rows, m.results[index+1:]...)...)
}

// collapse takes the versions of a group back off the list. They sit directly
// under their head and carry no count of their own, which is what says where
// the group ends.
func (m *Model) collapse(index int) {
	end := index + 1
	for end < len(m.results) && m.results[end].Group == m.results[index].Group &&
		!m.results[end].heads() {
		end++
	}

	m.results[index].Open = false
	m.results = append(m.results[:index+1], m.results[end:]...)
	m.found = clampFound(index, len(m.results))
}

// heading is the group row the cursor is on or under, and whether there is one
// at all: a version sits directly beneath the row it was opened out of, and a
// song with a single version is a group of nobody.
func (m *Model) heading() (int, bool) {
	if m.found < 0 || m.found >= len(m.results) {
		return 0, false
	}

	row := m.results[m.found]
	if row.heads() {
		return m.found, true
	}

	for index := m.found - 1; index >= 0; index-- {
		if m.results[index].Group != row.Group {
			return 0, false
		}
		if m.results[index].heads() {
			return index, true
		}
	}

	return 0, false
}

// sortByInstrument puts the tabs written for what is plugged in at the top.
// The site answers a guitar tab and a bass tab of the same song in one list,
// and the other one is somebody else's part.
func (m *Model) sortByInstrument() {
	want := tabsite.KindTab
	if m.instrument().Bass {
		want = tabsite.KindBass
	}

	sort.SliceStable(m.results, func(i, j int) bool {
		return m.results[i].Kind == want && m.results[j].Kind != want
	})
}

// showRecent is the left column: what has been read in and played, newest
// first. A song that was removed keeps its place, since the page it came from
// is still the answer to finding it again.
func (m *Model) showRecent() {
	m.kept = make([]finding, 0, len(m.recent.Entries))

	for _, entry := range m.recent.Entries {
		m.kept = append(m.kept, finding{
			From:    sourceRecent,
			Artist:  entry.Artist,
			Title:   entry.Title,
			Key:     songsterr.Key(entry.Artist, entry.Title),
			URL:     entry.URL,
			Path:    m.onDisk(entry),
			Version: entry.Version,
			Played:  entry.At,
		})
	}

	m.keptRow = clamp(m.keptRow, len(m.kept))
}

// musicRows is the list the keys are walking, since the screen draws two and
// only one of them has the cursor. Which one is what focus says.
func (m *Model) musicRows() []finding {
	if m.focus == paneRecent {
		return m.kept
	}
	return m.results
}

func (m *Model) musicCursor() int {
	if m.focus == paneRecent {
		return m.keptRow
	}
	return clampFound(m.found, len(m.results))
}

func (m *Model) setMusicCursor(row int) {
	if m.focus == paneRecent {
		m.keptRow = clamp(row, len(m.kept))
		return
	}
	m.found = clampFound(row, len(m.results))
}

// clampFound is the cursor of the search column, which has the field above its
// rows: fieldRow is where k off the top of the list lands, and it is where a
// search that answered nothing leaves the cursor.
func clampFound(row, length int) int {
	if row < fieldRow {
		return fieldRow
	}
	if row >= length {
		return length - 1
	}
	return row
}

// fieldRow is the row the search field is drawn on, above the first result.
const fieldRow = -1

// onDisk is where the song of an entry is, and nothing when the file has gone.
func (m *Model) onDisk(entry config.Entry) string {
	for _, item := range m.songs {
		if item.Path == entry.File {
			return item.Path
		}
	}
	return m.owned(entry.Artist, entry.Title)
}

// owned is the library file of a song, and nothing when it is not there. It is
// what puts the mark on a row, so the list says what you have without being a
// second screen.
func (m *Model) owned(artist, title string) string {
	for _, item := range m.songs {
		if strings.EqualFold(item.Title, title) && strings.EqualFold(item.Artist, artist) {
			return item.Path
		}
	}
	return ""
}

// lookupSongs asks songsterr what it calls the difficulty of every distinct
// song of a list. The list is handed in: the music screen and the spotify
// screen keep one each, and either of them can be waiting on an answer.
//
// Per song and not per row: a search answers a dozen versions of the one song
// and they all share the one answer. Three at a time and no more, because a
// playlist is hundreds of songs and firing hundreds of requests at once is how
// the search stops answering at all.
func (m *Model) lookupSongs(items []finding) tea.Cmd {
	type want struct {
		key    string
		artist string
		title  string
	}

	var songs []want
	seen := map[string]bool{}
	for _, item := range items {
		if item.State != lookupWaiting || seen[item.Key] {
			continue
		}
		seen[item.Key] = true
		songs = append(songs, want{key: item.Key, artist: item.Artist, title: item.Title})
	}

	client, family := m.songsterr, m.family()
	out := m.lookups

	return func() tea.Msg {
		gate := make(chan struct{}, 3)
		for _, asked := range songs {
			gate <- struct{}{}
			go func(asked want) {
				defer func() { <-gate }()

				found, err := client.Search(context.Background(), asked.artist+" "+asked.title)
				if err != nil {
					out <- lookupMsg{key: asked.key}
					return
				}

				// the artist and the title both, or the answer is about a
				// cover, a live version or another song by the same band
				best, ok := songsterr.Best(found, asked.artist, asked.title)
				if !ok {
					out <- lookupMsg{key: asked.key}
					return
				}

				out <- lookupMsg{key: asked.key, level: best.Difficulty(family), found: true}
			}(asked)
		}
		return nil
	}
}

func (m *Model) waitLookup() tea.Cmd {
	return func() tea.Msg { return <-m.lookups }
}

// answered fills in what songsterr said about one song, on every row of it and
// on both lists. A row is keyed by the artist and the title, so a song that is
// on the playlist and in a search has the one answer either way.
func (m *Model) answered(msg lookupMsg) {
	answerRows(m.results, msg)
	answerRows(m.tracks, msg)
}

func answerRows(items []finding, msg lookupMsg) {
	for index := range items {
		if items[index].Key != msg.key || items[index].State != lookupWaiting {
			continue
		}
		items[index].State = lookupMissing
		if msg.found {
			items[index].State, items[index].Level = lookupDone, msg.level
		}
	}
}

func stillLooking(items []finding) int {
	waiting := 0
	for _, item := range items {
		// a version of an opened song is the same song as the row above it and
		// is answered by the same lookup, so counting it counts twice
		if item.State == lookupWaiting && !item.under() {
			waiting++
		}
	}
	return waiting
}

// songsIn is how many songs a list holds, which is not how many rows it draws:
// a song answers a guitar row, a bass row and a chord sheet, and every one of
// them opens into versions of itself.
func songsIn(items []finding) int {
	songs := map[string]bool{}
	for _, item := range items {
		songs[item.Key] = true
	}
	return len(songs)
}

// grab reads a tab and writes it into the library. The text is in the page, so
// nothing goes through a browser and nothing has to be waited for in a
// downloads folder.
func (m *Model) grab(item finding) tea.Cmd {
	if item.URL == "" {
		m.fail = "that row carries no page to read"
		return nil
	}
	// a recent row carries no kind, and the page it names was read in already
	if item.Kind != "" && !tabsite.Playable(item.Kind) {
		m.fail = strings.ToLower(item.Kind) + " has no frets in it to read, only a tab does"
		return nil
	}

	dir, address := m.cfg.Library, item.URL
	client := siteOf(address, m.site)
	m.seeking = true
	m.status = "reading " + item.Title

	// the preview has usually read the page already, and it is the same page
	var known *tabsite.Tab
	if held := m.pages[address]; held != nil {
		known = held.tab
	}

	return func() tea.Msg {
		tab := known
		if tab == nil {
			read, err := client.Fetch(context.Background(), address)
			if err != nil {
				return grabMsg{err: err}
			}
			tab = read
		}

		// the tuning is kept beside the text there, not in it, so it is handed
		// to the parser rather than left to be guessed from the string count
		parsed, err := song.ParseTuned(tab.Text, tab.Title, tab.Tuning)
		if err != nil {
			return grabMsg{err: err}
		}

		parsed.Artist = tab.Artist

		// a site with one transcription a song numbers none of them
		name := fmt.Sprintf("%s %s", tab.Artist, tab.Title)
		if tab.Version > 0 {
			parsed.Track = fmt.Sprintf("version %d", tab.Version)
			name = fmt.Sprintf("%s v%d", name, tab.Version)
		}
		path, err := song.WriteAs(dir, parsed, name)
		if err != nil {
			return grabMsg{err: err}
		}

		return grabMsg{
			title:    parsed.Title,
			artist:   parsed.Artist,
			version:  tab.Version,
			url:      address,
			path:     path,
			notes:    len(parsed.Notes),
			measures: len(parsed.Measures),
		}
	}
}

// grabbed is a tab that landed: it goes on the recent list and the row says so.
func (m *Model) grabbed(msg grabMsg) tea.Cmd {
	m.recent.Remember(config.Entry{
		Artist:  msg.artist,
		Title:   msg.title,
		Version: msg.version,
		URL:     msg.url,
		File:    msg.path,
	})
	if err := m.recent.Save(); err != nil {
		m.fail = err.Error()
	}

	for index := range m.results {
		if m.results[index].URL == msg.url {
			m.results[index].Path = msg.path
		}
	}

	m.showRecent()
	m.status = fmt.Sprintf("%s read in, %d notes over %d measures, no rhythm in the source",
		msg.title, msg.notes, msg.measures)

	return m.loadSongs()
}

// practise opens a song that is already on disk, which is what enter means on
// every row that carries a file.
func (m *Model) practise(item finding) tea.Cmd {
	loaded := m.songAt(item.Path)
	if loaded == nil {
		m.fail = "the file for that one is not there any more"
		return nil
	}

	m.open(loaded)
	m.recent.Remember(item.entry())
	if err := m.recent.Save(); err != nil {
		m.fail = err.Error()
	}
	m.showRecent()

	return nil
}

func (m *Model) songAt(path string) *song.Song {
	for _, item := range m.songs {
		if item.Path == path {
			return item
		}
	}
	return nil
}

// remove is d. A song that is on disk is asked about first, since the file is
// the only copy of a tab that was read in; one that is only remembered goes
// without a word, because forgetting a line costs nothing.
func (m *Model) remove() tea.Cmd {
	items, row := m.musicRows(), m.musicCursor()
	if row < 0 || row >= len(items) {
		return nil
	}

	item := items[row]
	if item.Have() {
		m.removing, m.doomed = true, item
		m.status = "remove " + item.Title + "?  y / n"
		return nil
	}

	// a row of a search that was never read in has nothing behind it to
	// remove, and only the recent list keeps a line of its own
	if m.focus != paneRecent {
		m.status = "nothing of that one is here to remove"
		return nil
	}

	return m.forget(item)
}

// forget drops the song from the recent list, and the file with it when there
// is one. The two go together: a song with neither is not a song any more.
func (m *Model) forget(item finding) tea.Cmd {
	m.status = "forgot " + item.Title

	if item.Have() {
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			m.fail = err.Error()
			return nil
		}
		if m.current != nil && m.current.Path == item.Path {
			m.current, m.engine, m.tab = nil, nil, nil
		}
		m.dropSong(item.Path)
		m.status = "removed " + item.Title + " and its file"

		// the search beside the column is drawn from a list of its own and
		// would go on saying the song is here
		for index := range m.results {
			if m.results[index].Path == item.Path {
				m.results[index].Path = ""
			}
		}
	}

	m.recent.Forget(item.entry())
	if err := m.recent.Save(); err != nil {
		m.fail = err.Error()
	}
	m.showRecent()

	return m.loadSongs()
}

// dropSong takes a removed file off the library at once, so the row it was on
// does not go on saying the song is here until the folder is read again.
func (m *Model) dropSong(path string) {
	kept := make([]*song.Song, 0, len(m.songs))
	for _, item := range m.songs {
		if item.Path != path {
			kept = append(kept, item)
		}
	}
	m.songs = kept
}

func (m *Model) keySearch(key string) tea.Cmd {
	switch key {
	// the field is on the screen already, and i is what puts the cursor in it
	case "i":
		m.focus = paneSearch
		return m.ask(askingQuery, "artist and song, on "+m.cfg.Site, m.query)

	case "enter":
		return m.enterSearch()

	// l walks over to the search, and no further. reading a tab in and opening
	// a song are both decisions, and the key for a decision is enter
	case "l":
		if m.focus == paneRecent {
			m.focus = paneSearch
			return nil
		}
		m.status = "enter opens the row"
		return nil

	case "d":
		return m.remove()

	case "h", "esc":
		if m.focus == paneRecent {
			return nil
		}
		// h closes the group the cursor is in before it leaves the list, since
		// a key that opens something has to close it too
		if index, ok := m.heading(); ok && m.results[index].Open && key == "h" {
			m.collapse(index)
			return nil
		}

		// esc clears what was searched and h leaves it standing, since one of
		// them is done with the answer and the other is coming back to it
		m.focus = paneRecent
		if key == "esc" {
			m.query = ""
			m.results = nil
			m.found = fieldRow
		}
	}

	return nil
}

// enterSearch is the one key that means the same thing on either column: play
// the song when it is here, and go and get it when it is not. It is enter and
// not l, since it is the key that ends up starting a take.
func (m *Model) enterSearch() tea.Cmd {
	items, row := m.musicRows(), m.musicCursor()
	if row == fieldRow {
		return m.ask(askingQuery, "artist and song, on "+m.cfg.Site, m.query)
	}
	if row < 0 || row >= len(items) {
		return nil
	}

	item := items[row]

	// a song with more than one transcription of it opens into them first:
	// which version to read is a question and enter is what answers one. What
	// has no frets in it opens into nothing, since none of them can be read
	if item.heads() && tabsite.Playable(item.Kind) {
		m.expand(row)
		return nil
	}

	if item.Have() {
		return m.practise(item)
	}

	// a row with no page of its own is one that was played and then removed
	if item.URL == "" {
		return m.askSite(item.Artist + " " + item.Title)
	}

	return m.grab(item)
}

// askSite is the search itself, which is also what enter on a row with no
// page of its own falls back to. Which site it goes to is the config's, and
// nothing here asks anything else about it.
func (m *Model) askSite(pattern string) tea.Cmd {
	m.query = pattern
	m.focus = paneSearch
	m.results = nil
	m.found = fieldRow
	m.seeking = true
	m.status = "searching " + m.cfg.Site + " for " + pattern

	return m.searchSite(pattern)
}

// viewSearch is the two columns: what was played down the left and the search
// beside it, with one rule between them. Both are drawn every frame, and focus
// is the only thing that says which of them the keys are on.
func (m *Model) viewSearch() string {
	room := m.space()
	side := m.sidebarWidth()

	// the rule takes a column of its own between the two
	left := m.recentColumn(side, room)
	right := m.searchColumn(m.width-side-1, room)

	lines := make([]string, 0, room)
	for index := 0; index < room; index++ {
		lines = append(lines, rowPaint(false).fill(at(left, index), side)+
			styleRule.Render("\u2502")+at(right, index))
	}

	return strings.Join(lines, "\n")
}

// sidebarWidth is the column what was played is drawn in: a third of the
// window, held between the width a title needs and the width that would leave
// the search nothing.
func (m *Model) sidebarWidth() int {
	width := m.width / 3
	switch {
	case width < 22:
		width = 22
	case width > 38:
		width = 38
	}
	if width > m.width/2 {
		width = m.width / 2
	}
	return width
}

// at is one line of a column, and an empty one past the end of it: a column is
// as tall as the window and not as tall as what is in it.
func at(lines []string, index int) string {
	if index >= len(lines) {
		return ""
	}
	return lines[index]
}

// recentColumn is the left of the screen, and the way back to whatever was
// being worked on. It is drawn a column short of its width, so the rule beside
// it has a cell of its own and no row runs into it.
func (m *Model) recentColumn(width, room int) []string {
	width--
	lines := []string{"", columnHead("RECENTLY PLAYED", plural(len(m.kept), "song"), width), ""}

	start, end := window(m.keptRow, len(m.kept), (room-len(lines))/keptLines)
	m.clicks = append(m.clicks, clickable{top: headerLines + len(lines), first: start,
		count: end - start, width: width, step: keptLines, side: paneRecent})

	for index := start; index < end; index++ {
		lines = append(lines, m.recentRow(m.kept[index], index == m.keptRow, width)...)
	}

	return lines
}

// recentRow is one song of that column. The row under the cursor is painted
// only while the column has the keys, since two lit rows on one screen say
// nothing about which of them enter would open.
func (m *Model) recentRow(item finding, selected bool, width int) []string {
	paint := rowPaint(selected && m.focus == paneRecent)

	mark, title := "  ", styleInk
	if selected {
		title = styleHeading
		mark = paint.of(styleFaint).Render(" \u258e")
		if m.focus == paneRecent {
			mark = paint.of(styleAccent).Render(" \u258e")
		}
	}

	// a song whose file has gone keeps its line, and the line says so
	said := shortAgo(item.Played)
	if !item.Have() {
		said = "not here"
	}

	// the site the tab was read off, since a song is on this list whichever
	// one answered for it and neither can read the other's page
	tag, room := "", 3
	if letters := tabsite.TagOf(siteName(item.URL)); letters != "" {
		tag = paint.of(styleSubtle).Render(letters) + " "
		room += len(letters) + 1
	}

	head := mark + " " + paint.of(title).Render(item.Title)
	under := "   " + tag + paint.of(styleFaint).Render(item.Artist)

	return []string{
		paint.fill(truncate(head, width), width),
		paint.pad(truncate(under, width-len(said)-room),
			paint.of(styleFaint).Render(said+" "), width),
	}
}

// searchColumn is the right of the screen: the field, and under it whatever
// the last search answered.
func (m *Model) searchColumn(width, room int) []string {
	lead := m.searchBox(width)

	if m.seeking && len(m.results) == 0 {
		return m.spinnerLines(lead, "searching "+m.cfg.Site)
	}

	// a search that answered nothing says so, and a field nobody has typed in
	// yet says nothing at all: the placeholder on it is the whole of it
	if len(m.results) == 0 {
		if m.query == "" {
			return lead
		}
		return append(lead, "  "+styleSubtle.Render("Nothing came back for that."))
	}

	return m.listColumn(lead, "RESULTS", plural(songsIn(m.results), "song"),
		m.results, m.found, width, room, paneSearch)
}

// searchBox is the top of the right column. The field is on the screen whether
// or not it has the cursor, since a search nobody can see is a search nobody
// presses a key for, and i is what puts the cursor in it.
func (m *Model) searchBox(width int) []string {
	right := ""
	if m.input.Focused() {
		right = "enter searches, esc leaves it"
	}

	return []string{"", columnHead("SEARCH", right, width), "",
		truncate(m.searchField(), width), ""}
}

// searchField is the one line the query is typed on. What it holds does not
// move when the cursor arrives: the query that was searched is what it shows
// while it is blurred.
func (m *Model) searchField() string {
	caret := styleAccent.Render("  ▸")
	if m.input.Focused() {
		return caret + m.input.View()
	}

	// a cursor blinking on the field says the next key typed goes into it
	lead := "  "
	if m.onField() {
		lead = "  " + blinkCell()
	}

	if m.query == "" {
		return caret + lead + styleFaint.Render("press i and type an artist and a song")
	}

	return caret + lead + styleSubtle.Render(m.query)
}

// onField is the search column with the cursor above its rows, which is the
// field itself.
func (m *Model) onField() bool {
	return m.screen == screenMusic && m.focus == paneSearch &&
		clampFound(m.found, len(m.results)) == fieldRow
}

// blinkCell is the cursor of a field nobody is typing in yet. The view is
// redrawn every frame, so the clock is the whole of the blink.
func blinkCell() string {
	if time.Now().UnixMilli()/blinkRate%2 == 0 {
		return styleAccent.Render("█")
	}
	return " "
}

// blinkRate is how long the cursor is on, and then off, in milliseconds
const blinkRate = 500

// spinnerLines is the column while something is being waited for. The lead is
// whatever is drawn above it, since the music screen keeps its search field on
// while it waits and the spotify screen has no field at all.
func (m *Model) spinnerLines(lead []string, what string) []string {
	// the frame counter is the clock, so nothing has to be kept on the model
	frames := []string{"\u25d0", "\u25d3", "\u25d1", "\u25d2"}
	spin := frames[int(time.Now().UnixMilli()/120)%len(frames)]

	return append(lead, "  "+styleAccent.Render(spin)+"  "+styleSubtle.Render(what))
}

func (m *Model) viewSpinner(lead []string, what string) string {
	lines := m.spinnerLines(lead, what)
	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// listColumn is a list of songs drawn into a column, the one way wherever the
// rows came from: the music screen hands it a search and the spotify screen
// hands it a playlist, and a song looks the same on both.
func (m *Model) listColumn(lead []string, head, right string, items []finding,
	cursor, width, room int, side pane) []string {

	if len(items) == 0 {
		return append(lead, "  "+styleSubtle.Render("Nothing came back for that."))
	}

	if waiting := stillLooking(items); waiting > 0 {
		right = fmt.Sprintf("%s  \u00b7  %d still looking", right, waiting)
	}

	lines := append(lead, columnHead(head, right, width), "")

	// the bottom of the column is the tab of the row under the cursor, and the
	// list has what is left
	shown := m.viewPreview(width, room/2)

	start, end := window(cursor, len(items), room-len(lines)-len(shown))
	m.clicks = append(m.clicks, clickable{top: headerLines + len(lines), first: start,
		count: end - start, left: m.width - width, width: width, side: side})

	for index := start; index < end; index++ {
		lines = append(lines, m.findingRow(items[index], index == cursor, width))
	}

	for len(lines) < room-len(shown) {
		lines = append(lines, "")
	}

	return append(lines, shown...)
}

// viewFindings is that list over the whole window, which is what a screen
// drawing one column of it uses.
func (m *Model) viewFindings(lead []string, head, right string, items []finding, cursor int) string {
	lines := m.listColumn(lead, head, right, items, cursor, m.width, m.space(), paneSearch)
	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// window is the run of a long list that fits on the screen, kept around the
// cursor so walking down it scrolls instead of running off the bottom.
func window(cursor, count, room int) (int, int) {
	if room < 1 {
		room = 1
	}

	start := cursor - room/2
	if start > count-room {
		start = count - room
	}
	if start < 0 {
		start = 0
	}

	end := start + room
	if end > count {
		end = count
	}

	return start, end
}

// findingRow is the one row shared by the lists a site answered, so a song
// looks the same wherever it came from.
func (m *Model) findingRow(item finding, selected bool, width int) string {
	paint := rowPaint(selected)

	mark := "   "
	if selected {
		mark = paint.of(styleAccent).Render(" \u258e ")
	}

	left := mark + m.nameOf(item, paint, selected)
	right := m.rightOf(item, paint)

	return paint.pad(truncate(left, width-lipgloss.Width(right)-3),
		right+paint.of(styleFaint).Render(" "), width)
}

// nameOf is the left half of a row: the song with its artist beside it, and a
// version of an opened song written as the version it is, since the name of it
// is on the row it came out of.
func (m *Model) nameOf(item finding, paint rowPaint, selected bool) string {
	title := styleInk
	if selected {
		title = styleHeading
	}

	if item.under() {
		return paint.of(styleFaint).Render("    ") +
			paint.of(title).Render(fmt.Sprintf("version %d", item.Version))
	}

	// the arrow is what says a row opens and which way it is pointing now, and
	// the two cells it takes are kept on every row of the list so a song with
	// one transcription of it stands in the same column as the rest
	open := ""
	if item.From == sourceSearch {
		open = paint.of(styleFaint).Render("  ")
		if item.heads() && tabsite.Playable(item.Kind) {
			if item.Open {
				open = paint.of(styleAccent).Render("▾ ")
			} else {
				open = paint.of(styleAccent).Render("▸ ")
			}
		}
	}

	return open + paint.of(title).Render(item.Title) +
		paint.of(styleFaint).Render("   "+item.Artist)
}

// rightOf is what the site the row came from says about it, with the mark for
// a song that is already here in front of it.
func (m *Model) rightOf(item finding, paint rowPaint) string {
	right := m.saidAbout(item, paint)

	if item.Have() {
		right = paint.of(styleOk).Render("in library") + paint.of(styleFaint).Render("   ") + right
	}

	return right
}

// saidAbout draws what is known about a row. A recent one says when it was played,
// which is what that list is sorted by and the only question it answers.
func (m *Model) saidAbout(item finding, paint rowPaint) string {
	if item.Kind != "" && !tabsite.Playable(item.Kind) {
		return paint.of(styleFaint).Render(strings.ToLower(item.Kind))
	}

	written := m.versionOf(item, paint)

	// the difficulty is the song's and the row above carries it: printing it
	// again on every version of it says nothing new
	if item.under() {
		return written
	}

	if written != "" {
		written += paint.of(styleFaint).Render("   ")
	}

	switch item.State {
	case lookupWaiting:
		return written + paint.of(styleFaint).Render("  ·  ")
	case lookupMissing:
		// songsterr matched no song of that artist and title, and a blank
		// means not found rather than easy
		if item.From == sourceTracks {
			return paint.of(styleFaint).Render("no tab")
		}
		return written + paint.of(styleFaint).Render("     ")
	}

	if item.Level == 0 {
		return written + paint.of(styleFaint).Render("no guitar")
	}

	style := styleOk
	switch {
	case item.Level >= 6:
		style = styleBad
	case item.Level >= 4:
		style = styleWarn
	}

	return written + paint.of(style).Render(fmt.Sprintf("level %d", item.Level))
}

// versionOf is how one transcription is told from the next: a song there has a
// dozen of them and they differ by their number, by how many people rated one
// and by how well. A tab for the other instrument says so, since it is the one
// thing about a row that makes it somebody else's part.
func (m *Model) versionOf(item finding, paint rowPaint) string {
	if item.Kind == "" {
		return ""
	}

	// the number of a version is on the left of an opened row, so what is left
	// to say about it here is how people rated it
	written := paint.of(styleFaint).Render(fmt.Sprintf("v%d", item.Version))
	if item.under() {
		written = ""
	}
	if item.heads() {
		written = paint.of(styleSubtle).Render(plural(item.Count, "version"))
	}

	if (item.Kind == tabsite.KindBass) != m.instrument().Bass {
		instrument := paint.of(styleSubtle).Render(played(item.Kind))
		if written != "" {
			instrument += paint.of(styleFaint).Render("  ")
		}
		written = instrument + written
	}

	// how the best of them was rated is on the version itself once the row is
	// opened, and a count beside a rating beside a level is three numbers on a
	// row that answers one question
	if item.heads() {
		return written
	}

	if item.Votes == 0 {
		return written + paint.of(styleFaint).Render("   unrated")
	}

	style := styleWarn
	if item.Rating >= 4.5 {
		style = styleOk
	}

	return written + paint.of(styleFaint).Render("   ") +
		paint.of(style).Render(fmt.Sprintf("%.1f", item.Rating)) +
		paint.of(styleFaint).Render(fmt.Sprintf(" of %d", item.Votes))
}

// played is the instrument a kind of tab is for, in the one word that says it.
func played(kind string) string {
	if kind == tabsite.KindBass {
		return "bass"
	}
	return "guitar"
}

// shortAgo is how long since a song was played, in the two or three cells the
// column has for it.
func shortAgo(when time.Time) string {
	if when.IsZero() {
		return ""
	}

	since := time.Since(when)
	switch {
	case since < time.Minute:
		return "now"
	case since < time.Hour:
		return fmt.Sprintf("%dm", int(since.Minutes()))
	case since < 24*time.Hour:
		return fmt.Sprintf("%dh", int(since.Hours()))
	}

	return fmt.Sprintf("%dd", int(since.Hours()/24))
}
