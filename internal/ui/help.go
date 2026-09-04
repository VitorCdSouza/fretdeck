package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// binding is one line of the key map. The keys are written the way vim writes
// them, since that is what the whole interface is modelled on, and separated by
// a space: the brackets and the slash between two of them are drawn, not typed
// in here.
type binding struct {
	keys string
	what string
}

// global is the set that works on every screen. It is listed once, in the help,
// and nowhere else.
var global = []binding{
	{"H L", "previous/next screen"},
	{"i", "insert mode: type in the search field"},
	{"r", "repeat mode, on the practice screen: pick the measures to loop"},
	{"esc", "back to the normal mode"},
	{"?", "this help"},
	{"q", "quit"},
}

// bindings is what this screen does, most used first. The help behind the
// question mark is drawn from it and nothing else is, so a key that is not in
// here is a key nobody finds.
func (m *Model) bindings() []binding {
	if m.input.Focused() {
		return []binding{
			{"enter", "confirm"},
			{"esc", "cancel"},
		}
	}

	// the repeat mode is the practice screen with different keys, and the bar
	// is the only thing that says so besides the colour
	if m.mode == modeRepeat {
		return []binding{
			{"space", "repeat this measure, or stop"},
			{"h l", "note"},
			{"[ ]", "measure"},
			{"r", "let the passage go"},
			{"esc", "back to normal"},
		}
	}

	if m.removing {
		return []binding{
			{"y", "remove it"},
			{"n esc", "keep it"},
		}
	}

	switch m.screen {
	case screenMusic:
		if m.focus == paneRecent {
			return []binding{
				{"i", "search song"},
				{"j k", "up/down"},
				{"l", "over to the search"},
				{"enter", "practise it"},
				{"d", "remove it"},
				{"g G", "first/last"},
			}
		}
		return []binding{
			{"j k", "up/down, and the field over the first row"},
			{"enter", "type in the field, or open the row"},
			{"h", "close the versions, or back to what was played"},
			{"i", "search again"},
			{"d", "remove"},
			{"esc", "clear the search"},
			{"g G", "first/last"},
		}

	case screenPractice:
		return []binding{
			{"h l", "note"},
			{"[ ]", "measure"},
			{"r", "repeat a passage"},
			{"esc bksp", "back to the songs"},
		}

	case screenSpotify:
		switch m.stage {
		case stageLogin:
			return []binding{{"enter", "log in with spotify"}}
		case stagePlaylists:
			return []binding{
				{"j k", "up/down"},
				{"l enter", "open the playlist"},
				{"r", "read the library again"},
				{"g G", "first/last"},
			}
		}
		return []binding{
			{"j k", "up/down"},
			{"enter", "look for a tab of it"},
			{"h esc", "back to the playlists"},
			{"r", "another playlist"},
			{"g G", "first/last"},
		}

	case screenTuner:
		return []binding{{"", "play one string on its own"}}

	case screenConfig:
		if m.first == firstRunInstrument {
			return []binding{
				{"j k", "up/down"},
				{"l enter", "keep it and go on"},
				{"esc", "leave both for later"},
			}
		}
		if m.first == firstRunInput {
			return []binding{
				{"j k", "up/down"},
				{"l enter", "keep it and open the app"},
				{"r", "read the list again"},
				{"h", "back to the instrument"},
				{"esc", "leave it for later"},
			}
		}
		return []binding{
			{"j k", "up/down"},
			{"l enter", "keep it"},
			{"r", "read the inputs again"},
		}
	}

	return nil
}

// chip is one key and what it does, in the shape the bar at the bottom draws.
// The bar is the question mark and nothing else now: a screen with a dozen
// keys said them in a line and a half of chips, and a line and a half of the
// window is worth more than a map that is behind one key anyway.
// rowActions is what the row under the cursor offers, drawn in the bar beside
// the key map. It is the row and not the screen: a screen's whole map is
// behind the question mark, and the bar carries what the thing being looked at
// can be done to.
func (m *Model) rowActions() []binding {
	if m.input.Focused() || m.removing || m.helping {
		return nil
	}

	if m.screen == screenMusic && m.focus == paneRecent && m.musicCursor() < len(m.kept) {
		return []binding{{"d", "delete music"}}
	}

	return nil
}

func chip(item binding) string {
	return styleAccent.Render(keyLabel(item.keys)) + " " + styleFaint.Render(item.what)
}

// keyLabel is how a key is written everywhere it is offered: [i], and [j/k]
// when either of two does the same thing. The bar and the help both draw it
// here, so a screen that adds a binding writes no brackets of its own.
func keyLabel(keys string) string {
	parts := strings.Fields(keys)

	// the two that move a measure are the bracket keys themselves, and the
	// brackets around them need the room to be told apart
	for _, key := range parts {
		if key == "[" || key == "]" {
			return "[ " + strings.Join(parts, " / ") + " ]"
		}
	}

	return "[" + strings.Join(parts, "/") + "]"
}

// viewHelp is the whole map on one screen. It opens over whatever was there
// and the next key closes it, so it costs nothing to look at.
func (m *Model) viewHelp() string {
	head := []string{"", m.sectionHead("KEYS", screenNames[m.screen]), ""}

	tail := []string{"", "  " + styleAccent.Render("EVERYWHERE")}
	for _, item := range global {
		tail = append(tail, helpRow(item))
	}
	tail = append(tail,
		"",
		"  "+styleFaint.Render("movement is vim: j down, k up, h back, l in. any key closes this."),
		"  "+styleFaint.Render("the mouse selects a row and opens a screen, and the config screen turns it off."),
	)

	lines := append(head, m.helpRows(m.bindings(), m.space()-len(head)-len(tail))...)
	lines = append(lines, tail...)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// helpRows is the screen's own keys, one to a line, or two side by side when
// that is what it takes to fit the window. Nothing is ever left out: a key
// that is not on the map is a key nobody finds.
func (m *Model) helpRows(items []binding, room int) []string {
	half := m.width / 2

	if len(items) <= room || len(items) < 2 || half < 40 {
		rows := make([]string, 0, len(items))
		for _, item := range items {
			rows = append(rows, helpRow(item))
		}
		return rows
	}

	split := (len(items) + 1) / 2
	rows := make([]string, 0, split)

	for index := 0; index < split; index++ {
		row := helpRow(items[index])
		if index+split < len(items) {
			gap := half - lipgloss.Width(row)
			if gap < 2 {
				gap = 2
			}
			row += strings.Repeat(" ", gap) + helpRow(items[index+split])
		}
		rows = append(rows, truncate(row, m.width))
	}

	return rows
}

func helpRow(item binding) string {
	keys := styleAccent.Render(keyLabel(item.keys))
	gap := 14 - lipgloss.Width(keys)
	if gap < 1 {
		gap = 1
	}
	return "    " + keys + strings.Repeat(" ", gap) + styleSubtle.Render(item.what)
}
