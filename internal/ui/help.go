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
// and never repeated in the bar at the bottom.
var global = []binding{
	{"H L", "previous/next screen"},
	{"i", "insert mode: type in the search field"},
	{"r", "repeat mode, on the practice screen: pick the measures to loop"},
	{"esc", "back to the normal mode"},
	{"?", "this help"},
	{"q", "quit"},
}

// bindings is what this screen does, most used first. The first four go in the
// bar at the bottom, and all of them go in the help.
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
		if m.source == sourceRecent {
			return []binding{
				{"i", "search song"},
				{"j k", "up/down"},
				{"enter", "practise it"},
				{"d", "remove it"},
				{"g G", "first/last"},
			}
		}
		return []binding{
			{"j k", "up/down"},
			{"enter", "read it in, or practise it"},
			{"i", "search again"},
			{"d", "remove"},
			{"h esc", "what was played"},
			{"g G", "first/last"},
		}

	case screenPractice:
		return []binding{
			{"v", "tab or highway"},
			{"space", "run the clock"},
			{"m", "wait or tempo"},
			{"h l", "note"},
			{"[ ]", "measure"},
			{"+ -", "speed"},
			{"r", "repeat a passage"},
			{"R", "start over"},
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

// bar is the line at the bottom. Four keys is what fits without the eye
// skipping the lot, and the rest lives behind the question mark, which is why
// the question mark is the one chip that is never dropped: a key cut off the
// end of the line is a key nobody finds twice.
func (m *Model) bar(room int) string {
	shown := m.bindings()
	if len(shown) > 4 {
		shown = shown[:4]
	}

	help := chip(binding{"?", "keys"})
	gap := styleFaint.Render("   ")

	parts := make([]string, 0, len(shown)+1)
	width := lipgloss.Width(help)
	for _, item := range shown {
		next := chip(item)
		width += lipgloss.Width(next) + 3
		if width > room {
			break
		}
		parts = append(parts, next)
	}

	return strings.Join(append(parts, help), gap)
}

func chip(item binding) string {
	if item.keys == "" {
		return styleFaint.Render(item.what)
	}
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
	lines := []string{"", m.sectionHead("KEYS", screenNames[m.screen]), ""}

	for _, item := range m.bindings() {
		lines = append(lines, helpRow(item))
	}

	lines = append(lines, "", "  "+styleAccent.Render("EVERYWHERE"))
	for _, item := range global {
		lines = append(lines, helpRow(item))
	}

	lines = append(lines,
		"",
		"  "+styleFaint.Render("movement is vim: j down, k up, h back, l in. any key closes this."),
		"  "+styleFaint.Render("the mouse selects a row and opens a screen, and the config screen turns it off."),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func helpRow(item binding) string {
	keys := styleAccent.Render(keyLabel(item.keys))
	gap := 14 - lipgloss.Width(keys)
	if gap < 1 {
		gap = 1
	}
	return "    " + keys + strings.Repeat(" ", gap) + styleSubtle.Render(item.what)
}
