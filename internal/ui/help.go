package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// binding is one line of the key map. The keys are written the way vim writes
// them, since that is what the whole interface is modelled on.
type binding struct {
	keys string
	what string
}

// global is the set that works on every screen. It is listed once, in the help,
// and never repeated in the bar at the bottom.
var global = []binding{
	{"H L", "previous, next screen"},
	{"1 - 6", "jump to a screen"},
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

	switch m.screen {
	case screenLibrary:
		if m.tracks != nil {
			return []binding{
				{"j k", "track"},
				{"l enter", "import this one"},
				{"h esc", "back"},
			}
		}
		return []binding{
			{"j k", "song"},
			{"l enter", "practise it"},
			{"/", "filter"},
			{"i", "import a file"},
			{"g G", "first, last"},
			{"d", "remove"},
			{"r", "reload"},
		}

	case screenSearch:
		return []binding{
			{"/", "search songsterr"},
			{"j k", "result"},
			{"l enter", "open it to download"},
			{"g G", "first, last"},
		}

	case screenPractice:
		return []binding{
			{"v", "tab or highway"},
			{"space", "run the clock"},
			{"m", "wait or tempo"},
			{"h l", "note"},
			{"[ ]", "measure"},
			{"+ -", "speed"},
			{"r", "start over"},
			{"esc", "library"},
		}

	case screenTuner:
		return []binding{{"", "play one string on its own"}}

	case screenAnalyze:
		return []binding{
			{"a", "mark a recording"},
			{"j k", "scroll"},
			{"g G", "first, last"},
		}

	case screenSetup:
		return []binding{
			{"j k", "input"},
			{"l enter", "use it"},
			{"s", "connect spotify"},
		}
	}

	return nil
}

// bar is the line at the bottom. Four keys is what fits without the eye
// skipping the lot, and the rest lives behind the question mark.
func (m *Model) bar() string {
	shown := m.bindings()
	if len(shown) > 4 {
		shown = shown[:4]
	}

	parts := make([]string, 0, len(shown)+1)
	for _, item := range shown {
		parts = append(parts, chip(item))
	}
	parts = append(parts, chip(binding{"?", "keys"}))

	return strings.Join(parts, styleFaint.Render("   "))
}

func chip(item binding) string {
	if item.keys == "" {
		return styleFaint.Render(item.what)
	}
	return styleAccent.Render(item.keys) + " " + styleFaint.Render(item.what)
}

// viewHelp is the whole map on one screen. It opens over whatever was there
// and the next key closes it, so it costs nothing to look at.
func (m *Model) viewHelp() string {
	lines := []string{"", m.sectionHead("KEYS", screenNames[m.screen]), ""}

	for _, item := range m.bindings() {
		lines = append(lines, helpRow(item))
	}

	lines = append(lines, "", "  "+styleAccent.Render("EVERYWHERE"), "")
	for _, item := range global {
		lines = append(lines, helpRow(item))
	}

	lines = append(lines,
		"",
		"  "+styleFaint.Render("movement is vim: j down, k up, h back, l in."),
		"  "+styleFaint.Render("any key closes this."),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

func helpRow(item binding) string {
	keys := styleAccent.Render(item.keys)
	gap := 12 - lipgloss.Width(keys)
	if gap < 1 {
		gap = 1
	}
	return "    " + keys + strings.Repeat(" ", gap) + styleSubtle.Render(item.what)
}
