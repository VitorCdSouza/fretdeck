package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/VitorCdSouza/fretdeck/internal/tabsite"
)

// TestTheFirstLinesAreTheTabAndNotTheBlanksBeforeIt is what makes a pane of
// six lines six lines of tab.
func TestTheFirstLinesAreTheTabAndNotTheBlanksBeforeIt(t *testing.T) {
	text := "\n\n[Intro]\ne|--0--|\nB|--1--|\nG|--2--|\n"

	got := firstLines(text, 3)
	if len(got) != 3 {
		t.Fatalf("want three lines, got %d", len(got))
	}
	if got[0] != "[Intro]" {
		t.Fatalf("the blank lead was not cut, the first line is %q", got[0])
	}
}

// TestThePageIsReadOnceTheCursorHasComeToRest is what keeps a held down key
// from asking for twenty pages nobody looked at.
func TestThePageIsReadOnceTheCursorHasComeToRest(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = []finding{
		{Title: "one", Kind: tabsite.KindTab, URL: "a"},
		{Title: "two", Kind: tabsite.KindTab, URL: "b"},
	}

	now := time.Now()
	m.pointed, m.since = 0, now

	if cmd := m.resting(now.Add(rest / 2)); cmd != nil {
		t.Fatal("the cursor has not come to rest yet")
	}

	if cmd := m.resting(now.Add(2 * rest)); cmd == nil {
		t.Fatal("the cursor rested and no page was asked for")
	}
	if _, asked := m.pages["a"]; !asked {
		t.Fatal("the page was not marked as being read")
	}

	// and it is not asked for twice
	if cmd := m.resting(now.Add(3 * rest)); cmd != nil {
		t.Fatal("the same page was asked for again")
	}

	// moving the cursor starts the clock over
	m.found = 1
	if cmd := m.resting(now.Add(4 * rest)); cmd != nil {
		t.Fatal("a page was read the moment the cursor arrived")
	}
	if cmd := m.resting(now.Add(6 * rest)); cmd == nil {
		t.Fatal("the cursor rested on the second row and nothing was read")
	}
}

// TestThePaneSaysWhatItHas covers the three states it can be in, and the one
// row it is never drawn for.
func TestThePaneSaysWhatItHas(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneSearch
	m.results = []finding{{Title: "one", Kind: tabsite.KindTab, URL: "a"}}

	if lines := m.viewPreview(m.width, 12); len(lines) == 0 {
		t.Fatal("a row that is not here yet is what the pane is for")
	}

	m.pages = map[string]*page{"a": {}}
	if !strings.Contains(strings.Join(m.viewPreview(m.width, 12), " "), "reading the page") {
		t.Fatal("a page on its way has to say so")
	}

	m.pages["a"] = &page{tab: &tabsite.Tab{Text: "e|--0--|\n"}}
	if !strings.Contains(stripAnsi(strings.Join(m.viewPreview(m.width, 12), " ")), "e|--0--|") {
		t.Fatal("the tab did not make it onto the pane")
	}

	m.pages["a"] = &page{fail: "that page carries no tab"}
	if !strings.Contains(stripAnsi(strings.Join(m.viewPreview(m.width, 12), " ")), "no tab") {
		t.Fatal("a page that could not be read has to say why")
	}

	// a song that is already here is not previewed, it is here to be played
	m.results[0].Path = "/home/somebody/fretdeck/songs/one.json"
	if lines := m.viewPreview(m.width, 12); len(lines) != 0 {
		t.Fatal("a song in the library does not need looking at first")
	}
}

// TestARecentRowIsReadBackInWithoutAKind is the song that was removed and is
// being fetched again: only an ultimate guitar row says what kind it is.
func TestARecentRowIsReadBackInWithoutAKind(t *testing.T) {
	m := model(t)
	m.screen = screenMusic
	m.focus = paneRecent
	m.kept = []finding{{From: sourceRecent, Title: "otherside", URL: "a"}}

	if cmd := m.enterSearch(); cmd == nil {
		t.Fatal("the page was not read")
	}
	if m.fail != "" {
		t.Fatalf("a row with no kind was refused: %q", m.fail)
	}
}
