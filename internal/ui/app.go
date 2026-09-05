package ui

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
)

type screen int

const (
	screenMusic screen = iota
	screenPractice
	screenSpotify
	screenConfig
)

var screenNames = []string{"music", "practice", "spotify", "config"}

// tabScreens is what the navigation line has a button for. The practice screen
// is not one of them: it is a song open on the music screen, reached by opening
// a song and left with esc or backspace, so it is drawn as that screen and the
// line stays as short as the number of places there are to walk to.
var tabScreens = []screen{screenMusic, screenSpotify, screenConfig}

// headerLines is the name, the two rows a button takes and the rule under
// them. A click is answered by counting from it, so it is a constant and not a
// guess.
const headerLines = 4

// clickable is one run of rows on the screen: the line the first of them is on,
// the cursor index it stands for, and how many follow it. left and width are
// the columns it was drawn in and step is how many lines one of its rows
// takes, since the music screen draws two lists beside each other and the rows
// of one of them are two lines tall. A width of zero is the whole line.
type clickable struct {
	top   int
	first int
	count int
	left  int
	width int
	step  int
	side  pane
}

// frame is how often the clock and the animations are redrawn. Twenty five a
// second is smooth and is already faster than the twenty levels a second the
// worker sends, so nothing is drawn twice for nothing.
const frame = 40 * time.Millisecond

type bridgeMsg bridge.Event

// lateMsg is the device list not coming back. Without it a worker that dies
// between the asking and the answering leaves a screen reading forever.
type lateMsg struct{}
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

	// songs is the library on disk. It is not a screen any more: what it
	// answers is which row of a search is a song already here
	songs   []*song.Song
	current *song.Song

	// recent is what has been read in and played, which is what the search
	// screen shows with nothing typed
	recent config.Recent

	// removing says d was pressed on a song that is on disk and the question is
	// on the screen. Any key answers it, so the cursor cannot move under it,
	// and doomed is the song it was asked about
	removing bool
	doomed   finding

	// listing says the device list has been asked for and has not come back
	listing bool

	// helping puts the key map over whatever screen is open. It is a mode of
	// the whole app rather than of one screen, since the map is too
	helping bool

	// mode is normal, insert or repeat, and esc is the way back from either of
	// the two. It is the app's and not a screen's, since the palette follows it
	mode mode

	// pendingG is the first half of gg. Vim has a two key motion and so does
	// this, and the state has to live somewhere
	pendingG bool

	engine *practice.Engine
	tab    *song.Tab

	// the music screen, over a tab site and over what was played, with
	// songsterr answering for the difficulty beside a row
	// it is two lists side by side: kept is what was played, down the left, and
	// results is what the search answered. focus is the column the keys are on
	songsterr *songsterr.Client
	site      tabsite.Site
	query     string
	kept      []finding
	keptRow   int
	results   []finding
	found     int
	focus     pane
	seeking   bool
	lookups   chan lookupMsg

	// groups holds the versions of every song a search answered, keyed the way
	// the row on the list is. The list carries the best of each and the rest
	// are put under it when the row is opened
	groups map[string][]finding

	// the spotify screen, which is a step at a time: the login, the playlists
	// and the songs of the one that was opened. linked is whether there is a
	// session at all, read off the credentials file and not asked again while
	// the screen is being drawn
	linked    bool
	pulling   bool
	pulled    bool
	stage     spotifyStage
	picked    int
	playlist  string
	playlists []playlistInfo
	tracks    []finding

	level   levelPayload
	silence time.Time

	devices []deviceInfo

	// refreshed is the input the list was last read again for. A worker
	// reading something no list ever shows would otherwise ask forever
	refreshed string

	// configRow is the one cursor of the config screen. The instruments and the
	// inputs are two lists there and it walks both, so there is no focus to keep
	configRow int

	// first is which of the two first run questions is still open. An answer
	// that was kept is not asked about again
	first firstRun

	// clicks is where the rows of the list were drawn last, so a click can be
	// turned into the row under it. Only the drawing knows where a row landed
	clicks []clickable

	// the pages read for the preview, and the clock that says the cursor has
	// come to rest on a row long enough to be worth reading one
	pages   map[string]*page
	pointed int
	since   time.Time

	input  textinput.Model
	asking asking
}

// asking says what the one text field on screen is for. There is a single
// field for the whole app, since no screen ever needs two at once.
type asking int

const (
	askingNothing asking = iota
	askingQuery
)

// firstRun is what the config screen still has to ask. The two questions are asked in
// that order, and a run that has both answers already asks neither.
type firstRun int

const (
	firstRunDone firstRun = iota
	firstRunInstrument
	firstRunInput
)

// Options is what the command line can say. It is there so the app can be
// opened on a song, or on an input, without walking the screens to get there
// on every run while it is being worked on.
type Options struct {
	Song   string
	Device int
}

func New() *Model {
	return NewWith(Options{Device: -1})
}

func NewWith(options Options) *Model {
	input := textinput.New()
	input.Prompt = "  "
	input.CharLimit = 512

	cfg := config.Load()
	if options.Device >= 0 {
		// for one run and not kept: a flag is not somebody changing their mind
		cfg.Device, cfg.Source, cfg.Card = options.Device, "", ""
	}

	first, opening := firstRunDone, screenMusic
	switch {
	case cfg.Instrument == "":
		first, opening = firstRunInstrument, screenConfig
	case cfg.Device < 0:
		first, opening = firstRunInput, screenConfig
	}

	m := &Model{
		cfg:       cfg,
		recent:    config.LoadRecent(),
		worker:    bridge.NewWorker(),
		events:    make(chan bridge.Event, 256),
		lookups:   make(chan lookupMsg, 256),
		songsterr: songsterr.New(),
		site:      openSite(cfg.Site),
		input:     input,
		first:     first,
		screen:    opening,
		linked:    haveSession(),
	}
	if m.linked {
		m.stage = stagePlaylists
	}
	m.songsterr.Family = m.family()
	m.showRecent()

	if options.Song != "" {
		m.openPath(options.Song)
	}

	return m
}

// openPath opens a song by its path, which is what the flag gives. A path that
// does not read leaves the app where it was with the reason on the bar.
func (m *Model) openPath(path string) {
	loaded, err := song.Load(expand(path))
	if err != nil {
		m.fail = err.Error()
		return
	}

	m.open(loaded)
}

func (m *Model) Init() tea.Cmd {
	if err := m.worker.Start(); err != nil {
		m.fail = err.Error()
		return tea.Batch(m.waitWorker(), m.waitOneshot(), m.waitLookup(), m.loadSongs(), tick())
	}

	return tea.Batch(m.waitWorker(), m.waitOneshot(), m.waitLookup(), m.loadSongs(), tick(), m.askDevices())
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
		err := bridge.Run(context.Background(), script, args, events)

		// a script that ran and ended badly has already said why in an event of
		// its own, and only one that never ran at all is news
		var ended *exec.ExitError
		if err != nil && !errors.As(err, &ended) {
			events <- bridge.Event{Event: bridge.EventScriptGone, Message: err.Error()}
		}
		return nil
	}
}

// askDevices reads the input list off portaudio. It is asked for at startup
// and again on demand, since a device plugged in after the app started is on
// no list read before it.
//
// The worker is started again when it is not there. An empty list is what a
// worker that died looks like from here, and a refresh is what somebody
// presses when the list is empty.
func (m *Model) askDevices() tea.Cmd {
	if !m.worker.Running() {
		if err := m.worker.Start(); err != nil {
			m.fail = err.Error()
			return nil
		}
	}

	if err := m.worker.Send(bridge.Command{Action: "devices"}); err != nil {
		m.fail = err.Error()
		return nil
	}

	m.listing = true
	return tea.Tick(6*time.Second, func(time.Time) tea.Msg { return lateMsg{} })
}

func (m *Model) listen() tea.Cmd {
	command := bridge.Command{
		Action: "listen",
		Device: m.cfg.Device,
		Rate:   m.cfg.Rate,
		Source: m.cfg.Source,
		Card:   m.cfg.Card,
	}
	worker := m.worker
	return func() tea.Msg {
		_ = worker.Send(command)
		return nil
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// the field is in the right column and not across the window
		m.input.Width = msg.Width - m.sidebarWidth() - 8
		return m, nil

	case frameMsg:
		return m, tea.Batch(tick(), m.resting(time.Time(msg)))

	case songsMsg:
		m.songs = msg
		m.showRecent()
		m.markOwned()
		return m, nil

	case lateMsg:
		if m.listing {
			m.listing = false
			m.fail = "the audio worker did not answer. press r to ask it again"
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
		m.showTabs(msg.results)
		m.status = ""
		return m, m.lookupSongs(m.results)

	case grabMsg:
		m.seeking = false
		if msg.err != nil {
			m.fail = msg.err.Error()
			return m, nil
		}
		return m, m.grabbed(msg)

	case pageMsg:
		m.read(msg)
		return m, nil

	case lookupMsg:
		m.answered(msg)
		// the sort waits for the whole playlist: half a list is not an order
		if len(m.tracks) > 0 && stillLooking(m.tracks) == 0 {
			m.sortByLevel()
		}
		if stillLooking(m.results) == 0 && stillLooking(m.tracks) == 0 {
			m.status = ""
		}
		return m, m.waitLookup()

	case bridgeMsg:
		return m, m.handle(bridge.Event(msg))

	case tea.MouseMsg:
		return m, m.mouse(msg)

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
	case bridge.EventSpotifyLog, bridge.EventSpotifyReady, bridge.EventSpotifyError,
		bridge.EventSpotifyPlaylists, bridge.EventSpotifyTracks,
		bridge.EventScriptLog, bridge.EventScriptGone:
		next = m.waitOneshot()
	case bridge.EventLog:
		// a log of the worker's, since a one shot writes EventScriptLog. asking
		// the wrong mouth for the next event would hang that side forever
		next = m.waitWorker()
	}

	switch event.Event {
	case bridge.EventDevices:
		var payload devicesPayload
		if err := event.Decode(&payload); err == nil {
			m.devices = payload.Devices
			m.listing = false

			// nothing is opened while the first run is still asking
			if m.first == firstRunInstrument {
				return next
			}
			if m.first == firstRunInput {
				m.startingRow()
				m.status = plural(len(m.devices), "input") + " on the list"
				return next
			}

			m.configRow = clamp(m.configRow, m.configCount())
			if m.screen == screenConfig {
				m.status = plural(len(m.devices), "input") + " on the list"
			}

			found, ok := m.haveDevice()
			if !ok {
				m.lostDevice()
				if m.cfg.Source != "" || m.cfg.Card != "" {
					// the worker waits for it by name, so plugging it back in
					// is enough and there is nothing to press
					return tea.Batch(next, m.listen())
				}
				return next
			}

			m.cfg.Rate = found.Rate
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
		var payload listeningPayload
		if err := event.Decode(&payload); err == nil {
			m.fail = ""
			if again := m.keepInput(payload); again != nil {
				return tea.Batch(next, again)
			}
			m.status = "listening on " + m.deviceName()
		}

	case bridge.EventListenWaiting:
		// the worker is opening it again on its own, so this is what is
		// happening and not what went wrong
		m.status = event.Message

	case bridge.EventListenError, bridge.EventError, bridge.EventAudioWarning:
		m.listing = false
		m.fail = event.Message

	case bridge.EventScriptGone:
		m.pulling = false
		m.fail = event.Message

	case bridge.EventWorkerGone:
		m.listing = false
		m.devices = nil
		m.fail = event.Message + ". press r on the config screen to start it again"

	case bridge.EventSpotifyReady:
		m.linked = true
		m.status = "spotify connected"
		return tea.Batch(next, m.askPlaylists())

	case bridge.EventSpotifyLog:
		m.status = event.Message

	case bridge.EventSpotifyError:
		m.pulling = false
		m.fail = event.Message
		// the script answers this with no credentials file, and only the button can
		if !haveSession() {
			m.linked, m.stage = false, stageLogin
		}

	case bridge.EventSpotifyPlaylists:
		var payload playlistsPayload
		if err := event.Decode(&payload); err == nil {
			m.playlists = payload.Playlists
			m.picked = 0
			m.pulling = false
			m.status = ""
		}

	case bridge.EventSpotifyTracks:
		var payload tracksPayload
		if err := event.Decode(&payload); err == nil {
			return tea.Batch(next, m.showTracks(payload.Tracks))
		}

	}

	return next
}

// onNote is the whole point of the app: a pitch came in and the practice
// screen has to answer for it.
func (m *Model) onNote(note notePayload) {
	m.silence = time.Now()

	if m.engine == nil || m.screen != screenPractice {
		return
	}
	m.engine.Heard(note.Midi)
}

// chosen is whether a row of the list is the input that was kept. A name is
// what says so wherever the sound server gives one, since an index is
// renumbered by anything plugged in after the choice was made.
//
// The card answers when the name does not. A node carries the profile of its
// card in its name, so the same pedal is one name under duplex and another
// under pro audio, and the name that was kept is on no list until somebody
// switches the profile back by hand. That was the whole of the trouble.
func (m *Model) chosen(device deviceInfo) bool {
	if m.cfg.Source != "" || device.ID != "" {
		if device.ID == m.cfg.Source {
			return true
		}
		if m.listed(m.cfg.Source) {
			return false
		}
		if m.cfg.Card != "" && device.Card == m.cfg.Card {
			return true
		}
		return stem(m.cfg.Source) != "" && stem(device.ID) == stem(m.cfg.Source)
	}
	return device.Index == m.cfg.Device
}

// stem is a node name without the profile on the end of it, which is what an
// input kept before the card was written down is found by. A name of one part
// has none: bluez_input alone is every bluetooth input and not one of them.
func stem(name string) string {
	if strings.Count(name, ".") < 2 {
		return ""
	}
	return name[:strings.LastIndex(name, ".")]
}

// listed is whether the input that was kept is on the list under its own name.
func (m *Model) listed(source string) bool {
	if source == "" {
		return false
	}
	for _, device := range m.devices {
		if device.ID == source {
			return true
		}
	}
	return false
}

// keepInput writes down what the worker really opened. It is not always what
// it was asked for: a card in another profile is found by the card and answers
// under a name the config has never seen, and keeping that name is what stops
// the next run looking for one that is gone.
func (m *Model) keepInput(payload listeningPayload) tea.Cmd {
	if payload.Source == "" {
		return nil
	}

	if payload.Source != m.cfg.Source || payload.Card != m.cfg.Card {
		m.cfg.Source, m.cfg.Card = payload.Source, payload.Card
		if err := m.cfg.Save(); err != nil {
			m.fail = err.Error()
		}
	}

	// an input opened after the list was read is on no row of it, and the
	// screen would name something else while the guitar is being heard
	if m.listed(payload.Source) || m.refreshed == payload.Source {
		return nil
	}
	m.refreshed = payload.Source
	return m.askDevices()
}

// haveDevice is the input that was kept, when it is still on the list. The
// rate comes back with it, since the list is what says which one it takes.
func (m *Model) haveDevice() (deviceInfo, bool) {
	for _, device := range m.devices {
		if m.chosen(device) {
			return device, true
		}
	}
	return deviceInfo{}, false
}

// lostDevice is the one thing that reopens a question the first run has already
// answered. A portaudio index is not stable, so an interface plugged in after
// the app started renumbers the input that was chosen, and taking the system
// default without a word is how somebody ends up practising into the laptop
// microphone. The saved answer is left alone: plug the interface back in, read
// the list again and it is found where it was.
func (m *Model) lostDevice() {
	m.screen = screenConfig
	m.startingRow()
	m.fail = "the input you chose is not there. it is taken up when it comes back, or pick another one"
}

// instrument is what the config was told is plugged in. The tuner and the neck
// fall back on it whenever there is no song to read a tuning from.
func (m *Model) instrument() song.Instrument {
	return song.Chosen(m.cfg.Instrument)
}

// family is the half of the songsterr catalogue that answers for it.
func (m *Model) family() songsterr.Family {
	if m.instrument().Bass {
		return songsterr.Bass
	}
	return songsterr.Guitar
}

func (m *Model) deviceName() string {
	for _, device := range m.devices {
		if m.chosen(device) {
			return device.Name
		}
	}
	return "no input"
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

func credentialsPath() (string, error) {
	return config.Credentials()
}

func (m *Model) Close() {
	m.worker.Close()
}

// Mouse says whether the terminal should be asked for mouse events, which is
// what the program is started with.
func (m *Model) Mouse() bool { return m.cfg.Mouse }

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}

	m.clicks = nil

	if m.helping {
		return strings.Join([]string{m.header(), m.viewHelp(), m.footer()}, "\n")
	}

	body := ""
	switch m.screen {
	case screenMusic:
		body = m.viewSearch()
	case screenSpotify:
		body = m.viewSpotify()
	case screenPractice:
		body = m.viewPractice()
	case screenConfig:
		body = m.viewConfig()
	}

	return strings.Join([]string{m.header(), body, m.footer()}, "\n")
}

// header is what is the same on every screen: the name, and under it the
// buttons that walk between the screens.
func (m *Model) header() string {
	return styleBrand.Render("fretdeck") + "\n" + m.tabs() + "\n" + rule(m.width)
}

// tabs is the navigation line. Every screen is a button of its own and the two
// keys that walk between them sit at the edges, each on the side it walks
// towards.
func (m *Model) tabs() string {
	here := m.tabHere()

	boxes := make([]string, 0, 2*len(tabScreens)-1)
	for index, which := range tabScreens {
		box := styleTabBox.Render(styleTab.Render(screenNames[which]))
		if index == here {
			box = styleTabBoxOn.Render(styleTabOn.Render(screenNames[which]))
		}
		if index > 0 {
			boxes = append(boxes, strings.Repeat(" ", tabSpace))
		}
		boxes = append(boxes, box)
	}

	// the keys sit on the row the names are on, not on the row of the bars
	strip := lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		navKey("H"), tabGap, strip, tabGap, navKey("L"))

	return lipgloss.NewStyle().PaddingLeft(m.tabLead()).Render(line)
}

func navKey(letter string) string {
	return styleFaint.Render("[") + styleAccent.Render(letter) + styleFaint.Render("]")
}

// tabGap is what stands between a key and the buttons it walks through. The two
// belong to the strip and not to the edges of the window, so they sit against
// it and the whole of it is what is centred.
const tabGap = "  "

const (
	// tabPad is the space each side of a name, inside the border
	tabPad = 1
	// tabSpace is what stands between one button and the next, which is the
	// whole of what separates them now the border is under and not around
	tabSpace = 2
	// tabTop is the line of the window the buttons start on
	tabTop = 1
	// tabRows is the name and the bar under it, and both of them are clickable
	tabRows = 2
)

// tabWidth is a whole button: the name and the padding each side of it. The
// border is under it and takes no columns of its own.
func tabWidth(name string) int {
	return len(name) + 2*tabPad
}

// tabLead is how far in the navigation line starts. The drawing and the click
// both read it, so the two cannot drift apart.
func (m *Model) tabLead() int {
	whole := 2*(lipgloss.Width(navKey("H"))+len(tabGap)) + stripWidth()

	lead := (m.width - whole) / 2
	if lead < 0 {
		lead = 0
	}
	return lead
}

// stripWidth is the buttons and the space between them.
func stripWidth() int {
	width := 0
	for index, which := range tabScreens {
		if index > 0 {
			width += tabSpace
		}
		width += tabWidth(screenNames[which])
	}
	return width
}

// tabHere is the button that is lit. A song open lights the music screen: that
// is where it was opened from and where leaving it goes back to.
func (m *Model) tabHere() int {
	for index, which := range tabScreens {
		if which == m.screen {
			return index
		}
	}
	return 0
}

// screenAt is the screen whose button is under a column of the navigation line.
func (m *Model) screenAt(x int) (screen, bool) {
	at := m.tabLead() + lipgloss.Width(navKey("H")) + len(tabGap)
	for _, which := range tabScreens {
		width := tabWidth(screenNames[which])
		if x >= at && x < at+width {
			return which, true
		}
		at += width + tabSpace
	}
	return 0, false
}

// footer is one line: the way to the key map on the left and the input in the
// corner. An error takes the line over, since it is the only thing worth
// reading when there is one.
func (m *Model) footer() string {
	if m.fail != "" {
		return rule(m.width) + "\n" + styleBad.Render(truncate(m.fail, m.width))
	}

	// the question a removal asks takes the line over, since a file is the one
	// copy of a tab and nothing else on the bar is worth reading while it is on
	if m.removing {
		ask := styleWarn.Render("remove "+m.doomed.Title+"?") + chipGap +
			chip(binding{keys: "y", what: "remove it"}) + chipGap +
			chip(binding{keys: "n", what: "keep it"})
		return rule(m.width) + "\n" + truncate(ask, m.width)
	}

	keys := chip(binding{keys: "?", what: "keys"})
	for _, item := range m.rowActions() {
		keys += chipGap + chip(item)
	}

	return rule(m.width) + "\n" + truncate(pad(keys, m.hearing(), m.width), m.width)
}

// chipGap is what stands between one chip of the bar and the next
const chipGap = "   "

// hearing is the input and the note coming in on it, in the corner furthest
// from the tab, since it is read while nothing else is.
func (m *Model) hearing() string {
	text := styleFaint.Render("in ") + styleSubtle.Render(truncate(m.deviceName(), 20))
	if m.level.Freq > 0 && time.Since(m.silence) < 2*time.Second {
		text = styleOk.Render(m.level.Name) + "  " + text
	}
	return text
}

// space is how many lines the body may use, once the header and the two lines
// of the footer are taken out. The footer is two lines and never three now,
// which is the line the list got back.
func (m *Model) space() int {
	if m.height < 12 {
		return 6
	}
	return m.height - headerLines - 2
}

func blank(lines int) string {
	if lines < 1 {
		return ""
	}
	return strings.Repeat("\n", lines)
}

var _ = lipgloss.Width
