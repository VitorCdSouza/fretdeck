package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/ultimate"
)

// The bottom of the search column. A song there has a dozen
// transcriptions and the number beside a row only says how many people liked
// one; the first lines of the tab itself are what answers whether it is the
// one worth taking.
//
// The page is read once and kept, so walking back up the list costs nothing
// and enter on a row that has been looked at does not read it again.

// rest is how long the cursor has to sit on a row before its page is read. A
// key held down walks past twenty rows, and twenty pages is twenty requests
// for nineteen tabs nobody looked at.
const rest = 350 * time.Millisecond

// page is one transcription that has been read, or the reason it could not be.
// An entry with neither is one that is on its way.
type page struct {
	tab  *ultimate.Tab
	fail string
}

// pageMsg is a page that has come back.
type pageMsg struct {
	url string
	tab *ultimate.Tab
	err error
}

// resting keeps the clock on the cursor. The preview is asked for from the
// frame tick rather than from every key that moves, since the wheel, the
// click and j all move it and none of them should have to remember this.
func (m *Model) resting(now time.Time) tea.Cmd {
	if m.screen != screenMusic {
		return nil
	}

	if m.found != m.pointed {
		m.pointed, m.since = m.found, now
		return nil
	}
	if now.Sub(m.since) < rest {
		return nil
	}

	item, ok := m.previewOf()
	if !ok || m.pages[item.URL] != nil {
		return nil
	}

	if m.pages == nil {
		m.pages = map[string]*page{}
	}
	m.pages[item.URL] = &page{}

	return m.readPage(item.URL)
}

// previewOf is the row the preview is about, and whether there is one at all.
// A song that is already here is not previewed: it is here to be played.
func (m *Model) previewOf() (finding, bool) {
	// the music screen and no other: a spotify track carries no page to read
	if m.screen != screenMusic {
		return finding{}, false
	}
	if m.found >= len(m.results) {
		return finding{}, false
	}

	item := m.results[m.found]
	if item.URL == "" || item.Have() || !ultimate.Playable(item.Kind) && item.Kind != "" {
		return finding{}, false
	}

	return item, true
}

func (m *Model) readPage(address string) tea.Cmd {
	client := m.ultimate
	return func() tea.Msg {
		tab, err := client.Fetch(context.Background(), address)
		return pageMsg{url: address, tab: tab, err: err}
	}
}

func (m *Model) read(msg pageMsg) {
	kept := &page{tab: msg.tab}
	if msg.err != nil {
		kept = &page{fail: msg.err.Error()}
	}
	m.pages[msg.url] = kept
}

// viewPreview is the pane itself, and nothing at all when there is no row to
// preview, so a list of songs that are all here gets the whole column.
func (m *Model) viewPreview(width, room int) []string {
	item, ok := m.previewOf()
	if !ok || room < 5 {
		return nil
	}

	head := columnHead("THE TAB ITSELF", truncate(item.Title, width/3), width)
	lines := []string{rule(width), "", head, ""}

	for _, line := range m.previewBody(item, room-len(lines)) {
		lines = append(lines, "  "+styleFaint.Render(truncate(line, width-4)))
	}

	// the same height either way, or the list above would jump when it lands
	for len(lines) < room {
		lines = append(lines, "")
	}

	return lines
}

// previewBody is the first lines of the tab, or what is being done about not
// having them yet.
func (m *Model) previewBody(item finding, room int) []string {
	held, asked := m.pages[item.URL]
	switch {
	case !asked || (held.tab == nil && held.fail == ""):
		return []string{"reading the page"}
	case held.fail != "":
		return []string{held.fail}
	}

	return firstLines(held.tab.Text, room)
}

// firstLines is the top of the transcription with the empty lead cut off, so
// a pane of six lines is six lines of tab and not six blank ones.
func firstLines(text string, room int) []string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if len(kept) == 0 && strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
		if len(kept) == room {
			break
		}
	}
	return kept
}
