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

	"github.com/VitorCdSouza/fretdeck/internal/config"
	"github.com/VitorCdSouza/fretdeck/internal/song"
	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
	"github.com/VitorCdSouza/fretdeck/internal/ultimate"
)

// The one way in. Every song comes through here: the search reads tabs off
// ultimate guitar, and with nothing typed the screen is the list of what has
// been played, which is the way back to whatever was being worked on.
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
	sourceUltimate
	sourceTracks
)

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

	// Kind, Version, Rating and Votes are what an ultimate guitar row carries.
	// Played is what a row of the recent list carries instead
	Kind    string
	Version int
	Rating  float64
	Votes   int
	Played  time.Time
}

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

// ultimateMsg is a search, and grabMsg is one of its tabs read and written
// into the library, which is the whole of that trip.
type ultimateMsg struct {
	results []ultimate.Result
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

func (m *Model) searchUltimate(pattern string) tea.Cmd {
	client := m.ultimate
	return func() tea.Msg {
		results, err := client.Search(context.Background(), pattern)
		return ultimateMsg{results: results, err: err}
	}
}

// showTabs turns an ultimate guitar answer into rows. What cannot be read
// stays on the list rather than disappearing, since knowing a song is only up
// as chords is worth the line it takes.
func (m *Model) showTabs(results []ultimate.Result) {
	m.results = make([]finding, 0, len(results))
	for _, item := range results {
		m.results = append(m.results, finding{
			From:    sourceUltimate,
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
		})
	}

	m.sortByInstrument()
	m.found = 0
}

// sortByInstrument puts the tabs written for what is plugged in at the top.
// The site answers a guitar tab and a bass tab of the same song in one list,
// and the other one is somebody else's part.
func (m *Model) sortByInstrument() {
	want := ultimate.KindTab
	if m.instrument().Bass {
		want = ultimate.KindBass
	}

	sort.SliceStable(m.results, func(i, j int) bool {
		return m.results[i].Kind == want && m.results[j].Kind != want
	})
}

// showRecent is the screen with nothing typed: what has been read in and
// played, newest first. A song that was removed keeps its place, since the
// page it came from is still the answer to finding it again.
func (m *Model) showRecent() {
	m.source = sourceRecent
	m.results = make([]finding, 0, len(m.recent.Entries))

	for _, entry := range m.recent.Entries {
		m.results = append(m.results, finding{
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

	m.found = clamp(m.found, len(m.results))
}

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
		if item.State == lookupWaiting {
			waiting++
		}
	}
	return waiting
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
	if item.Kind != "" && !ultimate.Playable(item.Kind) {
		m.fail = strings.ToLower(item.Kind) + " has no frets in it to read, only a tab does"
		return nil
	}

	client, dir, address := m.ultimate, m.cfg.Library, item.URL
	m.seeking = true
	m.status = "reading " + item.Title

	// the preview has usually read the page already, and it is the same page
	var known *ultimate.Tab
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
		parsed.Track = fmt.Sprintf("version %d", tab.Version)

		name := fmt.Sprintf("%s %s v%d", tab.Artist, tab.Title, tab.Version)
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
	if m.found >= len(m.results) {
		return nil
	}

	item := m.results[m.found]
	if item.Have() {
		m.removing = true
		m.status = "remove " + item.Title + "?  y / n"
		return nil
	}

	// a row of a search that was never read in has nothing behind it to
	// remove, and only the recent list keeps a line of its own
	if m.source != sourceRecent {
		m.status = "nothing of that one is here to remove"
		return nil
	}

	return m.forget(m.found)
}

// forget drops the row from the recent list, and the file with it when there
// is one. The two go together: a song with neither is not a song any more.
func (m *Model) forget(row int) tea.Cmd {
	if row >= len(m.results) {
		return nil
	}

	item := m.results[row]
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
	}

	m.recent.Forget(item.entry())
	if err := m.recent.Save(); err != nil {
		m.fail = err.Error()
	}

	if m.source == sourceRecent {
		m.showRecent()
	} else {
		m.results[row].Path = ""
	}

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
		return m.ask(askingQuery, "artist and song, on ultimate guitar", m.query)

	case "enter":
		return m.enterSearch()

	// l walks into a list and no further. reading a tab in and opening a song
	// are both decisions, and the key for a decision is enter
	case "l":
		m.status = "enter opens the row"
		return nil

	case "d":
		return m.remove()

	case "h", "esc":
		if m.source == sourceUltimate {
			m.query = ""
			m.showRecent()
		}
	}

	return nil
}

// enterSearch is the one key that means the same thing on every list: play the
// song when it is here, and go and get it when it is not. It is enter and not
// l, since it is the key that ends up starting a take.
func (m *Model) enterSearch() tea.Cmd {
	if m.found >= len(m.results) {
		return nil
	}

	item := m.results[m.found]
	if item.Have() {
		return m.practise(item)
	}

	// a row with no page of its own is one that was played and then removed
	if item.URL == "" {
		return m.askUltimate(item.Artist + " " + item.Title)
	}

	return m.grab(item)
}

// askUltimate is the search itself, which is also what enter on a row with no
// page of its own falls back to.
func (m *Model) askUltimate(pattern string) tea.Cmd {
	m.query = pattern
	m.source = sourceUltimate
	m.results = nil
	m.found = 0
	m.seeking = true
	m.status = "searching ultimate guitar for " + pattern

	return m.searchUltimate(pattern)
}

func (m *Model) viewSearch() string {
	if m.seeking && len(m.results) == 0 {
		return m.viewSpinner(m.searchBox(), "searching ultimate guitar")
	}

	if m.source == sourceUltimate {
		return m.viewFindings(m.searchBox(), "RESULTS", fmt.Sprintf("%s  ·  %s",
			m.query, plural(len(m.results), "version")), m.results, m.found)
	}

	if len(m.results) == 0 {
		return m.viewSearchEmpty()
	}

	return m.viewFindings(m.searchBox(), "RECENTLY PLAYED",
		plural(len(m.results), "song"), m.results, m.found)
}

func (m *Model) viewSearchEmpty() string {
	lines := append(m.searchBox(),
		m.sectionHead("RECENTLY PLAYED", ""),
		"",
		"  "+styleSubtle.Render("Nothing has been played yet. Press "+styleAccent.Render("i")+styleSubtle.Render(" and type a song.")),
		"",
		"  "+styleSubtle.Render("Ultimate Guitar answers with every version of it people have written,"),
		"  "+styleSubtle.Render("and the text of the tab is in the page, so enter on one reads it into"),
		"  "+styleSubtle.Render("the library without leaving here. Songsterr says how hard it is."),
		"",
		"  "+styleSubtle.Render("The spotify screen reads a playlist of yours the same way, sorted"),
		"  "+styleSubtle.Render("from easiest to hardest, and enter on a song there searches here."),
		"",
		"  "+styleFaint.Render("What has been played shows up here, so this screen is the way back."),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// searchBox is the section at the top of the music screen. The field is on the
// screen whether or not it has the cursor, since a search nobody can see is a
// search nobody presses a key for, and i is what puts the cursor in it.
func (m *Model) searchBox() []string {
	right := ""
	if m.input.Focused() {
		right = "enter searches, esc leaves it"
	}

	return []string{"", m.sectionHead("SEARCH", right), "", m.searchField(), ""}
}

// searchField is the one line the query is typed on. What it holds does not
// move when the cursor arrives: the query that was searched is what it shows
// while it is blurred.
func (m *Model) searchField() string {
	caret := styleAccent.Render("  ▸")
	if m.input.Focused() {
		return caret + m.input.View()
	}
	if m.query == "" {
		return caret + styleFaint.Render("  press i and type an artist and a song")
	}

	return caret + styleSubtle.Render("  "+m.query)
}

// viewSpinner is the screen while something is being waited for. The lead is
// whatever the screen draws above it, since the music screen keeps its search
// field on while it waits and the spotify screen has no field at all.
func (m *Model) viewSpinner(lead []string, what string) string {
	// the frame counter is the clock, so nothing has to be kept on the model
	frames := []string{"◐", "◓", "◑", "◒"}
	spin := frames[int(time.Now().UnixMilli()/120)%len(frames)]

	lines := append(lead, "  "+styleAccent.Render(spin)+"  "+styleSubtle.Render(what))

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// viewFindings is the list of songs, drawn the one way wherever the rows came
// from: the music screen hands it a search and the spotify screen hands it a
// playlist, and a song looks the same on both.
func (m *Model) viewFindings(lead []string, head, right string, items []finding, cursor int) string {
	if len(items) == 0 {
		lines := append(lead, styleSubtle.Render("  Nothing came back for that."))
		return strings.Join(lines, "\n") + blank(m.space()-len(lines))
	}

	if waiting := stillLooking(items); waiting > 0 {
		right = fmt.Sprintf("%s  ·  %d still looking", right, waiting)
	}

	lines := append(lead, m.sectionHead(head, right), "")

	// the bottom half of the screen is the tab of the row under the cursor,
	// and the list has what is left
	pane := m.viewPreview(m.space() / 2)

	start, end := window(cursor, len(items), m.space()-len(lines)-len(pane))
	m.clicks = []clickable{{top: headerLines + len(lines), first: start, count: end - start}}

	for index := start; index < end; index++ {
		lines = append(lines, m.findingRow(items[index], index == cursor))
	}

	for len(lines) < m.space()-len(pane) {
		lines = append(lines, "")
	}
	lines = append(lines, pane...)

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

// findingRow is the one row shared by the three lists, so a song looks the
// same wherever it came from.
func (m *Model) findingRow(item finding, selected bool) string {
	paint := rowPaint(selected)

	mark, title := "   ", paint.of(styleInk).Render(item.Title)
	if selected {
		mark, title = paint.of(styleAccent).Render(" ▎ "), paint.of(styleHeading).Render(item.Title)
	}

	left := mark + title + paint.of(styleFaint).Render("   "+item.Artist)
	right := m.rightOf(item, paint)

	return paint.pad(truncate(left, m.width-lipgloss.Width(right)-3),
		right+paint.of(styleFaint).Render(" "), m.width)
}

// rightOf is what the site the row came from says about it, with the mark for
// a song that is already here in front of it.
func (m *Model) rightOf(item finding, paint rowPaint) string {
	right := m.saidAbout(item, paint)

	switch {
	case item.Have():
		right = paint.of(styleOk).Render("in library") + paint.of(styleFaint).Render("   ") + right
	case item.From == sourceRecent:
		right = paint.of(styleFaint).Render("not here") + paint.of(styleFaint).Render("   ") + right
	}

	return right
}

// saidAbout draws what is known about a row. A recent one says when it was played,
// which is what that list is sorted by and the only question it answers.
func (m *Model) saidAbout(item finding, paint rowPaint) string {
	if item.From == sourceRecent {
		return paint.of(styleFaint).Render(ago(item.Played))
	}
	if item.Kind != "" && !ultimate.Playable(item.Kind) {
		return paint.of(styleFaint).Render(strings.ToLower(item.Kind))
	}

	written := m.versionOf(item, paint)
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

	written := paint.of(styleFaint).Render(fmt.Sprintf("v%d", item.Version))
	if (item.Kind == ultimate.KindBass) != m.instrument().Bass {
		written = paint.of(styleSubtle).Render(played(item.Kind)) +
			paint.of(styleFaint).Render("  "+written)
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
	if kind == ultimate.KindBass {
		return "bass"
	}
	return "guitar"
}

// ago is how long since a song was played, in the one unit that answers.
func ago(when time.Time) string {
	if when.IsZero() {
		return ""
	}

	since := time.Since(when)
	switch {
	case since < time.Minute:
		return "just now"
	case since < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(since.Minutes()))
	case since < 24*time.Hour:
		return plural(int(since.Hours()), "hour") + " ago"
	case since < 48*time.Hour:
		return "yesterday"
	}

	return plural(int(since.Hours()/24), "day") + " ago"
}
