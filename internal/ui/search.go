package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
)

// source says what the list on the search screen is showing. All three produce
// the same kind of row, which is why they share a screen: a song, how hard
// songsterr calls it, and whether it is already in the library.
type source int

const (
	sourceSongsterr source = iota
	sourcePlaylists
	sourceTracks
)

// finding is one row. It comes either from a songsterr search, where the
// difficulty is known at once, or from a spotify playlist, where every row has
// to be looked up before it can say anything.
type finding struct {
	Artist string
	Title  string
	Level  int
	URL    string
	Have   bool
	State  lookup
}

type lookup int

const (
	lookupDone lookup = iota
	lookupWaiting
	lookupMissing
)

type playlistInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type playlistsPayload struct {
	Playlists []playlistInfo `json:"playlists"`
}

type spotifyTrack struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
}

type tracksPayload2 struct {
	Tracks []spotifyTrack `json:"tracks"`
}

// found messages. The search and the lookups are both slow and both have to
// arrive without the interface waiting on them.
type searchMsg struct {
	songs []songsterr.Song
	err   error
}

type lookupMsg struct {
	index int
	level int
	url   string
	found bool
}

// watchMsg carries the state of the wait for a downloaded file, so the poll can
// re-issue itself without any of it living on the model.
type watchMsg struct {
	dir      string
	since    time.Time
	deadline time.Time
	path     string
	done     bool
}

func (m *Model) searchSongsterr(pattern string) tea.Cmd {
	client := m.songsterr
	return func() tea.Msg {
		songs, err := client.Search(context.Background(), pattern)
		return searchMsg{songs: songs, err: err}
	}
}

// showSongs turns a songsterr answer into rows. A song written for no guitar
// keeps its place at the bottom of the list rather than disappearing, since the
// artist may still be the one that was typed.
func (m *Model) showSongs(songs []songsterr.Song) {
	m.results = make([]finding, 0, len(songs))
	for _, item := range songs {
		m.results = append(m.results, finding{
			Artist: item.Artist,
			Title:  item.Title,
			Level:  item.Difficulty(),
			URL:    item.URL(),
			Have:   m.owned(item.Artist, item.Title),
		})
	}
	m.found = 0
}

// owned answers whether the library already holds that song, so the list can
// say so instead of sending somebody to download what they have.
func (m *Model) owned(artist, title string) bool {
	for _, item := range m.songs {
		if strings.EqualFold(item.Title, title) && strings.EqualFold(item.Artist, artist) {
			return true
		}
	}
	return false
}

// lookupTracks asks songsterr about every track of a playlist.
//
// Three at a time and no more. A playlist is hundreds of songs, and firing
// hundreds of requests at once is how a search stops answering at all.
func (m *Model) lookupTracks(tracks []spotifyTrack) tea.Cmd {
	client := m.songsterr
	out := m.lookups

	return func() tea.Msg {
		gate := make(chan struct{}, 3)
		for index, track := range tracks {
			gate <- struct{}{}
			go func(index int, track spotifyTrack) {
				defer func() { <-gate }()

				songs, err := client.Search(context.Background(), track.Artist+" "+track.Title)
				if err != nil {
					out <- lookupMsg{index: index}
					return
				}

				best, ok := songsterr.Best(songs, track.Artist, track.Title)
				if !ok {
					out <- lookupMsg{index: index}
					return
				}

				out <- lookupMsg{index: index, level: best.Difficulty(), url: best.URL(), found: true}
			}(index, track)
		}
		return nil
	}
}

func (m *Model) waitLookup() tea.Cmd {
	return func() tea.Msg { return <-m.lookups }
}

// open hands the page to the browser and starts watching for the file. The tab
// itself cannot be fetched from here, so this is the whole of what the app can
// do about getting one.
func (m *Model) open(item finding) tea.Cmd {
	if item.URL == "" {
		m.fail = "there is no songsterr page for that one"
		return nil
	}

	if err := browse(item.URL); err != nil {
		m.fail = err.Error()
		return nil
	}

	m.status = "opened " + item.Title + ", watching " + m.cfg.Downloads
	return watchDownloads(m.cfg.Downloads, time.Now(), time.Now().Add(10*time.Minute))
}

func browse(url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, url).Start()
}

// watchDownloads polls for a guitar pro file that was not there before.
//
// Polling rather than watching the folder: one stat a second costs nothing and
// it saves a dependency whose only job would be to tell us the same thing.
func watchDownloads(dir string, since, deadline time.Time) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		if path, ok := newestTab(dir, since); ok {
			return watchMsg{path: path, done: true}
		}
		if time.Now().After(deadline) {
			return watchMsg{done: true}
		}
		return watchMsg{dir: dir, since: since, deadline: deadline}
	})
}

var tabExtensions = map[string]bool{".gp": true, ".gp3": true, ".gp4": true, ".gp5": true, ".gpx": true}

// newestTab is the most recent guitar pro file written since the wait started.
// A file still being written has its size checked twice a second apart by the
// caller coming back, so a half downloaded file is not imported.
func newestTab(dir string, since time.Time) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	best, when := "", since
	for _, entry := range entries {
		if entry.IsDir() || !tabExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().After(when) {
			continue
		}
		best, when = filepath.Join(dir, entry.Name()), info.ModTime()
	}

	return best, best != ""
}

func (m *Model) keySearch(key string) tea.Cmd {
	switch key {
	case "/":
		return m.ask(askingQuery, "artist and song, or either on its own", m.query)

	case "s":
		m.source = sourcePlaylists
		m.results = nil
		m.playlists = nil
		m.seeking = true
		m.status = "reading your spotify library"
		return m.spotify("playlists")

	case "l", "enter":
		return m.enterSearch()

	case "h", "esc":
		if m.source == sourceTracks {
			m.source = sourcePlaylists
			m.results = nil
			m.found = 0
		}
	}

	return nil
}

func (m *Model) enterSearch() tea.Cmd {
	if m.source == sourcePlaylists {
		if m.found >= len(m.playlists) {
			return nil
		}
		chosen := m.playlists[m.found]
		m.source = sourceTracks
		m.playlist = chosen.Name
		m.results = nil
		m.seeking = true
		m.status = "reading " + chosen.Name
		return m.spotify("tracks", "--playlist", chosen.ID)
	}

	if m.found >= len(m.results) {
		return nil
	}

	return m.open(m.results[m.found])
}

func (m *Model) spotify(action string, args ...string) tea.Cmd {
	path, err := credentialsPath()
	if err != nil {
		m.fail = err.Error()
		return nil
	}
	return m.run("spotify.py", append([]string{action, "--credentials", path}, args...)...)
}

// sortByLevel puts the easy songs at the top, which is the question somebody
// brings a playlist here to ask. What was not found sinks to the bottom.
func (m *Model) sortByLevel() {
	sort.SliceStable(m.results, func(i, j int) bool {
		left, right := m.results[i], m.results[j]
		if (left.State == lookupDone) != (right.State == lookupDone) {
			return left.State == lookupDone
		}
		return left.Level < right.Level
	})
}

func (m *Model) viewSearch() string {
	if m.asking == askingQuery {
		return m.viewAsk("Search songsterr",
			"It answers which songs have a tab and how hard they call the guitar\n"+
				"part. The file itself is downloaded from the page, which opens in\n"+
				"your browser, and lands in the library on its own.")
	}

	switch m.source {
	case sourcePlaylists:
		return m.viewPlaylists()
	case sourceTracks:
		return m.viewFindings(m.playlist)
	}

	if m.query == "" {
		return m.viewSearchEmpty()
	}

	return m.viewFindings(fmt.Sprintf("%s  ·  %s", m.query, plural(len(m.results), "result")))
}

func (m *Model) viewSearchEmpty() string {
	lines := []string{
		"",
		m.sectionHead("SEARCH", ""),
		"",
		"  " + styleSubtle.Render("Press "+styleAccent.Render("/")+styleSubtle.Render(" and type a song. Songsterr answers which ones have a")),
		"  " + styleSubtle.Render("tab and how hard the guitar part is."),
		"",
		"  " + styleSubtle.Render("Press "+styleAccent.Render("s")+styleSubtle.Render(" to pull a playlist out of Spotify instead, and see the")),
		"  " + styleSubtle.Render("whole thing sorted from easiest to hardest."),
		"",
		"  " + styleFaint.Render("Enter on a result opens its page in the browser. Download the"),
		"  " + styleFaint.Render("Guitar Pro file there and it imports itself."),
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewPlaylists() string {
	if m.seeking && len(m.playlists) == 0 {
		return m.viewSpinner("reading your spotify library")
	}
	if len(m.playlists) == 0 {
		return m.viewNoSpotify()
	}

	lines := []string{"", m.sectionHead("SPOTIFY", plural(len(m.playlists), "playlist")), ""}

	for index, item := range m.playlists {
		mark, name := "   ", styleInk.Render(item.Name)
		if index == m.found {
			mark, name = styleAccent.Render(" ▎ "), styleHeading.Render(item.Name)
		}
		lines = append(lines, pad(mark+name, styleFaint.Render(plural(item.Count, "song")), m.width))
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewNoSpotify() string {
	lines := []string{
		"",
		m.sectionHead("SPOTIFY", ""),
		"",
		"  " + styleSubtle.Render("Not connected yet. The setup screen has the login, under "+styleAccent.Render("s")+styleSubtle.Render(".")),
		"",
		"  " + styleFaint.Render("It opens spotify in your browser and keeps the session here."),
		"  " + styleFaint.Render("No app to register, no client id to paste."),
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewSpinner(what string) string {
	// the frame counter is the clock, so nothing has to be kept on the model
	frames := []string{"◐", "◓", "◑", "◒"}
	spin := frames[int(time.Now().UnixMilli()/120)%len(frames)]

	lines := []string{"", "  " + styleAccent.Render(spin) + "  " + styleSubtle.Render(what)}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewFindings(head string) string {
	if m.seeking && len(m.results) == 0 {
		return m.viewSpinner(head)
	}
	if len(m.results) == 0 {
		return "\n" + styleSubtle.Render("  Nothing came back for that.") + blank(m.space()-2)
	}

	waiting := 0
	for _, item := range m.results {
		if item.State == lookupWaiting {
			waiting++
		}
	}

	right := head
	if waiting > 0 {
		right = fmt.Sprintf("%s  ·  %d still looking", head, waiting)
	}

	lines := []string{"", m.sectionHead("RESULTS", right), ""}

	room := m.space() - len(lines)
	start := m.found - room/2
	if start < 0 {
		start = 0
	}
	end := start + room
	if end > len(m.results) {
		end = len(m.results)
	}

	for index := start; index < end; index++ {
		lines = append(lines, m.findingRow(m.results[index], index == m.found))
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// findingRow is the one row shared by the search and the playlist, so a song
// looks the same wherever it came from.
func (m *Model) findingRow(item finding, selected bool) string {
	mark, title := "   ", styleInk.Render(item.Title)
	if selected {
		mark, title = styleAccent.Render(" ▎ "), styleHeading.Render(item.Title)
	}

	left := mark + title + styleFaint.Render("   "+item.Artist)
	right := level(item)

	if item.Have {
		right = styleOk.Render("in library") + styleFaint.Render("   ") + right
	}

	return pad(truncate(left, m.width-lipgloss.Width(right)-3), right+" ", m.width)
}

// level draws what songsterr calls the difficulty. Their scale runs past five,
// so the number is printed as it is and the colour carries the meaning: it
// would take inventing a ceiling to turn it into stars.
func level(item finding) string {
	switch item.State {
	case lookupWaiting:
		return styleFaint.Render("  ·  ")
	case lookupMissing:
		return styleFaint.Render("no tab")
	}

	if item.Level == 0 {
		return styleFaint.Render("no guitar")
	}

	style := styleOk
	switch {
	case item.Level >= 6:
		style = styleBad
	case item.Level >= 4:
		style = styleWarn
	}

	return style.Render(fmt.Sprintf("level %d", item.Level))
}
