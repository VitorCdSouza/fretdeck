package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/config"
	"github.com/VitorCdSouza/fretdeck/internal/practice"
	"github.com/VitorCdSouza/fretdeck/internal/song"
	"github.com/VitorCdSouza/fretdeck/internal/songsterr"
)

type screen int

const (
	screenLibrary screen = iota
	screenSearch
	screenPractice
	screenTuner
	screenAnalyze
	screenSetup
)

var screenNames = []string{"library", "search", "practice", "tuner", "analyze", "setup"}

// frame is how often the clock and the animations are redrawn. Twenty five a
// second is smooth and is already faster than the twenty levels a second the
// worker sends, so nothing is drawn twice for nothing.
const frame = 40 * time.Millisecond

type bridgeMsg bridge.Event
type frameMsg time.Time
type songsMsg []*song.Song
type errMsg struct{ text string }
type statusMsg struct{ text string }

// Model is the whole app. Bubble Tea wants one, and splitting it across files
// by screen keeps it readable without pretending each screen is independent:
// they share the worker, the config and the library.
type Model struct {
	cfg    config.Config
	worker *bridge.Worker
	events chan bridge.Event

	screen screen
	width  int
	height int
	status string
	fail   string

	songs   []*song.Song
	pick    int
	current *song.Song

	// filter is the vim style search over the library, kept apart from the
	// songsterr one so leaving the screen does not lose either
	filter string

	// helping puts the key map over whatever screen is open. It is a mode of
	// the whole app rather than of one screen, since the map is too
	helping bool

	// pendingG is the first half of gg. Vim has a two key motion and so does
	// this, and the state has to live somewhere
	pendingG bool

	engine *practice.Engine
	tab    *song.Tab
	heard  notePayload

	// highway is the guitar hero view of the same song. It is a way of
	// drawing what the engine already knows, not a mode of its own
	highway bool

	// the search screen, over songsterr and over a spotify playlist
	songsterr *songsterr.Client
	query     string
	results   []finding
	found     int
	seeking   bool
	source    source
	playlist  string
	playlists []playlistInfo
	lookups   chan lookupMsg

	level   levelPayload
	silence time.Time

	devices []deviceInfo
	device  int

	input     textinput.Model
	asking    asking
	tracks    []trackInfo
	track     int
	pending   string
	report    *reportPayload
	progress  float64
	running   bool
	reportRow int
}

// asking says what the one text field on screen is for. There is a single
// field for the whole app, since no screen ever needs two at once.
type asking int

const (
	askingNothing asking = iota
	askingImport
	askingRecording
	askingQuery
	askingFilter
)

func New() *Model {
	input := textinput.New()
	input.Prompt = "  "
	input.CharLimit = 512

	cfg := config.Load()

	return &Model{
		cfg:       cfg,
		worker:    bridge.NewWorker(),
		events:    make(chan bridge.Event, 256),
		lookups:   make(chan lookupMsg, 256),
		songsterr: songsterr.New(),
		device:    cfg.Device,
		input:     input,
	}
}

func (m *Model) Init() tea.Cmd {
	if err := m.worker.Start(); err != nil {
		m.fail = err.Error()
	} else {
		_ = m.worker.Send(bridge.Command{Action: "devices"})
	}

	return tea.Batch(m.waitWorker(), m.waitOneshot(), m.waitLookup(), m.loadSongs(), tick())
}

// waitWorker and waitOneshot are two mouths feeding the same Update. The live
// worker never ends and the one shot scripts come and go, and keeping them
// apart means an import cannot interrupt the note stream.
func (m *Model) waitWorker() tea.Cmd {
	return func() tea.Msg { return bridgeMsg(<-m.worker.Events) }
}

func (m *Model) waitOneshot() tea.Cmd {
	return func() tea.Msg { return bridgeMsg(<-m.events) }
}

func tick() tea.Cmd {
	return tea.Tick(frame, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func (m *Model) loadSongs() tea.Cmd {
	dir := m.cfg.Library
	return func() tea.Msg {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errMsg{err.Error()}
		}
		loaded, err := song.List(dir)
		if err != nil {
			return errMsg{err.Error()}
		}
		return songsMsg(loaded)
	}
}

// run starts a one shot script and keeps feeding its events into the same
// channel until it ends, so the screen sees progress while it is still going.
func (m *Model) run(script string, args ...string) tea.Cmd {
	events := m.events
	return func() tea.Msg {
		if err := bridge.Run(context.Background(), script, args, events); err != nil {
			events <- bridge.Event{Event: bridge.EventLog, Message: err.Error()}
		}
		return nil
	}
}

func (m *Model) listen() tea.Cmd {
	device, rate := m.cfg.Device, m.cfg.Rate
	worker := m.worker
	return func() tea.Msg {
		_ = worker.Send(bridge.Command{Action: "listen", Device: device, Rate: rate})
		return nil
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = msg.Width - 6
		return m, nil

	case frameMsg:
		if m.engine != nil {
			m.engine.Tick(time.Time(msg))
		}
		return m, tick()

	case songsMsg:
		m.songs = msg
		if m.pick >= len(m.songs) {
			m.pick = 0
		}
		return m, nil

	case statusMsg:
		m.status = msg.text
		return m, nil

	case errMsg:
		m.fail = msg.text
		return m, nil

	case searchMsg:
		m.seeking = false
		if msg.err != nil {
			m.fail = msg.err.Error()
			return m, nil
		}
		m.showSongs(msg.songs)
		m.status = ""
		return m, nil

	case lookupMsg:
		if msg.index < len(m.results) {
			item := &m.results[msg.index]
			item.State = lookupMissing
			if msg.found {
				item.State, item.Level, item.URL = lookupDone, msg.level, msg.url
			}
		}
		if m.stillLooking() == 0 {
			m.sortByLevel()
			m.status = ""
		}
		return m, m.waitLookup()

	case watchMsg:
		return m, m.watched(msg)

	case bridgeMsg:
		return m, m.handle(bridge.Event(msg))

	case tea.KeyMsg:
		return m, m.key(msg)
	}

	return m, nil
}

// handle turns one event from python into a change on screen. The command it
// returns is always the wait for the next event on the same mouth, or the
// stream stops.
func (m *Model) handle(event bridge.Event) tea.Cmd {
	next := m.waitWorker()
	switch event.Event {
	case bridge.EventTracks, bridge.EventImported, bridge.EventImportError,
		bridge.EventProgress, bridge.EventReport, bridge.EventAnalyzeError,
		bridge.EventSpotifyLog, bridge.EventSpotifyReady, bridge.EventSpotifyError,
		bridge.EventSpotifyPlaylists, bridge.EventSpotifyTracks:
		next = m.waitOneshot()
	case bridge.EventLog:
		// a log can come from either mouth, and asking the wrong one for the
		// next event would hang that side forever. the live worker is the one
		// that must never stall, so it gets the benefit of the doubt
		next = m.waitWorker()
	}

	switch event.Event {
	case bridge.EventDevices:
		var payload devicesPayload
		if err := event.Decode(&payload); err == nil {
			m.devices = payload.Devices
			m.adoptDevice()
			return tea.Batch(next, m.listen())
		}

	case bridge.EventNote:
		var payload notePayload
		if err := event.Decode(&payload); err == nil {
			m.onNote(payload)
		}

	case bridge.EventLevel:
		var payload levelPayload
		if err := event.Decode(&payload); err == nil {
			m.level = payload
		}

	case bridge.EventListening:
		m.status = "listening on " + m.deviceName(m.cfg.Device)

	case bridge.EventListenError, bridge.EventError, bridge.EventAudioWarning:
		m.fail = event.Message

	case bridge.EventTracks:
		var payload tracksPayload
		if err := event.Decode(&payload); err == nil {
			m.tracks = payload.Tracks
			m.track = firstPlayable(payload.Tracks)
			m.asking = askingNothing
			m.input.Blur()
		}

	case bridge.EventImported:
		var payload importedPayload
		if err := event.Decode(&payload); err == nil {
			m.tracks = nil
			m.status = fmt.Sprintf("%s imported, %d notes over %d measures",
				payload.Title, payload.Notes, payload.Measures)
			return tea.Batch(next, m.loadSongs())
		}

	case bridge.EventImportError, bridge.EventAnalyzeError:
		m.fail = event.Message
		m.running = false
		m.tracks = nil

	case bridge.EventSpotifyReady:
		m.status = "spotify connected"

	case bridge.EventSpotifyLog:
		m.status = event.Message

	case bridge.EventSpotifyError:
		m.seeking = false
		m.fail = event.Message

	case bridge.EventSpotifyPlaylists:
		var payload playlistsPayload
		if err := event.Decode(&payload); err == nil {
			m.playlists = payload.Playlists
			m.found = 0
			m.seeking = false
			m.status = ""
		}

	case bridge.EventSpotifyTracks:
		var payload tracksPayload2
		if err := event.Decode(&payload); err == nil {
			return tea.Batch(next, m.showTracks(payload.Tracks))
		}

	case bridge.EventProgress:
		var payload progressPayload
		if err := event.Decode(&payload); err == nil && payload.Total > 0 {
			m.progress = float64(payload.Done) / float64(payload.Total)
		}

	case bridge.EventReport:
		var payload reportPayload
		if err := event.Decode(&payload); err == nil {
			m.report = &payload
			m.reportRow = 0
			m.running = false
			m.progress = 1
		}
	}

	return next
}

// onNote is the whole point of the app: a pitch came in and the practice
// screen has to answer for it.
func (m *Model) onNote(note notePayload) {
	m.heard = note
	m.silence = time.Now()

	if m.engine == nil || m.screen != screenPractice {
		return
	}
	if m.engine.Mode == practice.Tempo && !m.engine.Running() {
		return
	}
	m.engine.Heard(note.Midi, time.Now())
}

// adoptDevice keeps the config pointing at something that exists. A device
// index is not stable across reboots, so the saved one is checked and the
// system default is taken when it is gone.
func (m *Model) adoptDevice() {
	for _, device := range m.devices {
		if device.Index == m.cfg.Device {
			return
		}
	}
	for _, device := range m.devices {
		if device.Default {
			m.cfg.Device = device.Index
			m.cfg.Rate = device.Rate
			return
		}
	}
	if len(m.devices) > 0 {
		m.cfg.Device = m.devices[0].Index
		m.cfg.Rate = m.devices[0].Rate
	}
}

func (m *Model) deviceName(index int) string {
	for _, device := range m.devices {
		if device.Index == index {
			return device.Name
		}
	}
	return "no input"
}

func firstPlayable(tracks []trackInfo) int {
	for index, track := range tracks {
		if track.Playable {
			return index
		}
	}
	return 0
}

// expand turns a path typed with a tilde into one the file system knows.
func expand(path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// showTracks turns a playlist into rows and sends every one of them off to be
// looked up. Nothing is known about difficulty until songsterr answers.
func (m *Model) showTracks(tracks []spotifyTrack) tea.Cmd {
	m.seeking = false
	m.results = make([]finding, 0, len(tracks))
	for _, track := range tracks {
		m.results = append(m.results, finding{
			Artist: track.Artist,
			Title:  track.Title,
			State:  lookupWaiting,
			Have:   m.owned(track.Artist, track.Title),
		})
	}
	m.found = 0
	m.status = fmt.Sprintf("looking %d up on songsterr", len(tracks))

	return m.lookupTracks(tracks)
}

func (m *Model) stillLooking() int {
	waiting := 0
	for _, item := range m.results {
		if item.State == lookupWaiting {
			waiting++
		}
	}
	return waiting
}

// watched keeps the poll going until a file shows up or the wait runs out. A
// file that does show up is read for its tracks, and the library screen picks
// up the question of which one to import.
func (m *Model) watched(msg watchMsg) tea.Cmd {
	if !msg.done {
		return watchDownloads(msg.dir, msg.since, msg.deadline)
	}
	if msg.path == "" {
		m.status = "stopped watching for a download"
		return nil
	}

	m.pending = msg.path
	m.screen = screenLibrary
	m.status = "found " + filepath.Base(msg.path)

	return m.run("gpimport.py", msg.path)
}

// problemCount is how many rows the analyze screen can scroll through, which
// is only what went wrong.
func (m *Model) problemCount() int {
	if m.report == nil {
		return 0
	}

	count := 0
	for _, note := range m.report.Notes {
		if note.Kind != "hit" {
			count++
		}
	}
	return count
}

func credentialsPath() (string, error) {
	return config.Credentials()
}

func (m *Model) Close() {
	m.worker.Close()
}

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.helping {
		return strings.Join([]string{m.header(), m.viewHelp(), m.footer()}, "\n")
	}

	body := ""
	switch m.screen {
	case screenLibrary:
		body = m.viewLibrary()
	case screenSearch:
		body = m.viewSearch()
	case screenPractice:
		body = m.viewPractice()
	case screenTuner:
		body = m.viewTuner()
	case screenAnalyze:
		body = m.viewAnalyze()
	case screenSetup:
		body = m.viewSetup()
	}

	return strings.Join([]string{m.header(), body, m.footer()}, "\n")
}

// header is the one line that is the same on every screen: the name, where you
// are, and what the app is hearing right now.
func (m *Model) header() string {
	tabs := make([]string, len(screenNames))
	for index, name := range screenNames {
		if screen(index) == m.screen {
			tabs[index] = styleTabOn.Render(name)
			continue
		}
		tabs[index] = styleTab.Render(name)
	}

	left := styleBrand.Render("fretdeck") + "  " +
		strings.Join(tabs, styleFaint.Render(" · "))

	right := styleFaint.Render("in ") + styleSubtle.Render(truncate(m.deviceName(m.cfg.Device), 28))
	if m.level.Freq > 0 && time.Since(m.silence) < 2*time.Second {
		right = styleOk.Render(m.level.Name) + "  " + right
	}

	return pad(left, right, m.width) + "\n" + rule(m.width)
}

// footer carries the keys of the screen on the left and whatever the app has
// to say on the right. An error takes the line over, since it is the only
// thing worth reading when there is one.
func (m *Model) footer() string {
	if m.fail != "" {
		return rule(m.width) + "\n" + styleBad.Render(truncate(m.fail, m.width))
	}

	keys := m.bar()
	if m.status == "" {
		return rule(m.width) + "\n" + truncate(keys, m.width)
	}

	return rule(m.width) + "\n" +
		pad(keys, styleSubtle.Render(truncate(m.status, m.width/2)), m.width)
}

// space is how many lines the body may use, once the two lines of the header
// and the two of the footer are taken out.
func (m *Model) space() int {
	if m.height < 10 {
		return 6
	}
	return m.height - 5
}

func blank(lines int) string {
	if lines < 1 {
		return ""
	}
	return strings.Repeat("\n", lines)
}

var _ = lipgloss.Width
