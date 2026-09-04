package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
)

// The spotify screen is one question at a time. Logged out it is a single
// button, since there is nothing else that can be done about it; logged in it
// is the playlists, and a playlist opened is its songs sorted by how hard
// songsterr says they are on the instrument the config screen was told about.
//
// Nothing is read in from here. A track carries an artist and a title and no
// tab at all, so enter on one is the search on the music screen, which is the
// one way a song gets into the library.

// spotifyStage is what the screen is showing. The three follow one another and
// h walks back through them.
type spotifyStage int

const (
	stageLogin spotifyStage = iota
	stagePlaylists
	stageTracks
)

// playlistInfo is one row of the library. The liked songs come back as one of
// these too, first and under an id of the script's own, since spotify keeps
// them in a collection of their own and the screen should not have to know.
//
// How many songs are in one is not on it: the answer that names every playlist
// in a single request does not carry a length, and asking each of them for one
// would be a request a playlist to fill in a number the next screen says anyway.
type playlistInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type playlistsPayload struct {
	Playlists []playlistInfo `json:"playlists"`
}

// spotifyTrack is one song of a playlist, which is an artist and a title and
// nothing else.
type spotifyTrack struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
}

type tracksPayload struct {
	Tracks []spotifyTrack `json:"tracks"`
}

// haveSession is whether the login has been done. The credentials file is what
// says so, here and in the script both, so the two cannot disagree. It is read
// when the app starts and when a login answers, and never while drawing: the
// screen is redrawn twenty five times a second.
func haveSession() bool {
	path, err := credentialsPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// enterSpotify is what opening the screen does. The library is read the first
// time it is opened with a session, so it is there without anything to press,
// and not again on every walk past it.
func (m *Model) enterSpotify() tea.Cmd {
	if !m.linked {
		m.stage = stageLogin
		return nil
	}
	if m.stage == stageLogin {
		m.stage = stagePlaylists
	}
	// once a run: spotify rate limits the client id every login shares
	if m.pulling || m.pulled {
		return nil
	}
	return m.askPlaylists()
}

// login opens the browser. librespot is what runs it and what writes the
// credentials file when it comes back, and that file is the whole of what this
// screen turns on.
func (m *Model) login() tea.Cmd {
	if m.pulling {
		return nil
	}
	m.status = "opening the spotify login in your browser"
	return m.pull(m.spotify("login"))
}

func (m *Model) askPlaylists() tea.Cmd {
	m.stage = stagePlaylists
	m.picked = 0
	m.pulled = true
	m.status = "reading your spotify library"
	return m.pull(m.spotify("playlists"))
}

// pull marks the screen as waiting on the script, unless there was nothing to
// wait for because the script could not be started at all.
func (m *Model) pull(cmd tea.Cmd) tea.Cmd {
	m.pulling = cmd != nil
	return cmd
}

func (m *Model) spotify(action string, args ...string) tea.Cmd {
	path, err := credentialsPath()
	if err != nil {
		m.fail = err.Error()
		return nil
	}
	return m.run("spotify.py", append([]string{action, "--credentials", path}, args...)...)
}

// showTracks turns a playlist into rows and sends every one of them off to be
// looked up. Nothing is known about the difficulty until songsterr answers,
// and the sort waits for the lot, since half a list is not an order.
func (m *Model) showTracks(tracks []spotifyTrack) tea.Cmd {
	m.pulling = false
	m.tracks = make([]finding, 0, len(tracks))

	for _, track := range tracks {
		m.tracks = append(m.tracks, finding{
			From:   sourceTracks,
			Artist: track.Artist,
			Title:  track.Title,
			Key:    songsterr.Key(track.Artist, track.Title),
			State:  lookupWaiting,
			Path:   m.owned(track.Artist, track.Title),
		})
	}

	m.picked = 0
	m.status = fmt.Sprintf("looking %d up for the %s", len(tracks), m.instrument().Name)

	return m.lookupSongs(m.tracks)
}

// sortByLevel puts the easy songs at the top, which is the question somebody
// brings a playlist here to ask. What was not found sinks to the bottom.
func (m *Model) sortByLevel() {
	sort.SliceStable(m.tracks, func(i, j int) bool {
		left, right := m.tracks[i], m.tracks[j]
		if (left.State == lookupDone) != (right.State == lookupDone) {
			return left.State == lookupDone
		}
		return left.Level < right.Level
	})
}

// lookupAgain asks about a playlist that is already on the screen. The number
// beside a row is the difficulty of the instrument being played, so changing
// the instrument changes every one of them and the order they are in.
func (m *Model) lookupAgain() tea.Cmd {
	if len(m.tracks) == 0 {
		return nil
	}

	for index := range m.tracks {
		m.tracks[index].State = lookupWaiting
	}
	m.status = "looking the playlist up again for the " + m.instrument().Name

	return m.lookupSongs(m.tracks)
}

// markOwned says which rows of the playlist are songs that are here now. The
// library is read again whenever a tab lands, and a row that was searched for
// and read in has to stop saying it is missing.
func (m *Model) markOwned() {
	for index := range m.tracks {
		m.tracks[index].Path = m.owned(m.tracks[index].Artist, m.tracks[index].Title)
	}
}

// spotifyRows is how many rows the screen is showing, which is the one cursor
// it has walking over the playlists or over the songs of one.
func (m *Model) spotifyRows() int {
	switch m.stage {
	case stagePlaylists:
		return len(m.playlists)
	case stageTracks:
		return len(m.tracks)
	}
	return 0
}

func (m *Model) keySpotify(key string) tea.Cmd {
	switch key {
	case "enter":
		return m.pressSpotify()

	// l walks into a list and no further, and finding a tab is a decision
	case "l":
		if m.stage == stageTracks {
			m.status = "enter looks for a tab of it"
			return nil
		}
		return m.pressSpotify()

	case "h", "esc":
		if m.stage == stageTracks {
			m.stage = stagePlaylists
			m.playlist = ""
			m.tracks = nil
			m.picked = 0
		}
		return nil

	case "r":
		// a playlist made on the phone is on no list read before it
		if m.stage == stageLogin || m.pulling {
			return nil
		}
		m.playlists, m.tracks, m.playlist = nil, nil, ""
		return m.askPlaylists()
	}

	return nil
}

// pressSpotify is enter, which means one thing on each step: log in, open the
// playlist, go and look for a tab of the song.
func (m *Model) pressSpotify() tea.Cmd {
	switch m.stage {
	case stageLogin:
		return m.login()

	case stagePlaylists:
		if m.picked >= len(m.playlists) {
			return nil
		}
		chosen := m.playlists[m.picked]
		m.stage = stageTracks
		m.playlist = chosen.Name
		m.tracks = nil
		m.picked = 0
		m.status = "reading " + chosen.Name
		return m.pull(m.spotify("tracks", "--playlist", chosen.ID))
	}

	if m.picked >= len(m.tracks) {
		return nil
	}

	item := m.tracks[m.picked]
	if item.Have() {
		return m.practise(item)
	}

	// spotify knows an artist and a title, so enter is the search for them
	return tea.Batch(m.goTo(screenMusic), m.askSite(item.Artist+" "+item.Title))
}

func (m *Model) viewSpotify() string {
	if !m.linked {
		return m.viewLogin()
	}
	if m.stage == stageTracks {
		return m.viewTracks()
	}
	return m.viewPlaylists()
}

// spotifyHead is the lead every step of the screen starts with, so the button
// and the two lists all land on the same line.
func (m *Model) spotifyHead(right string) []string {
	return []string{"", m.sectionHead("SPOTIFY", right), ""}
}

func (m *Model) viewLogin() string {
	if m.pulling {
		return m.viewSpinner(m.spotifyHead(""), "waiting for the login in your browser")
	}

	button := loginButton()
	lines := m.spotifyHead("not connected")
	m.clicks = []clickable{{top: headerLines + len(lines), count: len(button)}}
	lines = append(lines, button...)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// loginButton is the one button in the app. Every other key is offered on the
// bar at the bottom, and a screen whose whole content is a single press has to
// show the press.
func loginButton() []string {
	label := styleHeading.Render("Log in with Spotify") +
		styleFaint.Render("   ") + styleAccent.Render(keyLabel("enter"))

	return strings.Split(lipgloss.NewStyle().MarginLeft(2).Render(styleButton.Render(label)), "\n")
}

func (m *Model) viewPlaylists() string {
	if m.pulling && len(m.playlists) == 0 {
		return m.viewSpinner(m.spotifyHead(""), "reading your spotify library")
	}

	if len(m.playlists) == 0 {
		lines := append(m.spotifyHead(""),
			"  "+styleSubtle.Render("Nothing came back. Press "+styleAccent.Render("r")+styleSubtle.Render(" to read the library again.")))
		return strings.Join(lines, "\n") + blank(m.space()-len(lines))
	}

	lines := m.spotifyHead(plural(len(m.playlists), "playlist"))

	start, end := window(m.picked, len(m.playlists), m.space()-len(lines))
	m.clicks = []clickable{{top: headerLines + len(lines), first: start, count: end - start}}

	for index := start; index < end; index++ {
		item := m.playlists[index]
		mark, name := "   ", styleInk.Render(item.Name)
		if index == m.picked {
			mark, name = styleAccent.Render(" ▎ "), styleHeading.Render(item.Name)
		}
		lines = append(lines, mark+name)
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func (m *Model) viewTracks() string {
	if m.pulling && len(m.tracks) == 0 {
		return m.viewSpinner(m.spotifyHead(m.playlist), "reading "+m.playlist)
	}

	right := fmt.Sprintf("%s  ·  %s", m.playlist, plural(len(m.tracks), "song"))

	return m.viewFindings([]string{""}, "SPOTIFY", right, m.tracks, m.picked)
}
