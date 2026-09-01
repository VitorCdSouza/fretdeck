package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/VitorCdSouza/fretdeck/internal/song"
)

func textinputBlink() tea.Cmd { return textinput.Blink }

func (m *Model) viewLibrary() string {
	if m.asking == askingImport {
		return m.viewAsk("Import a Guitar Pro file",
			"The tab, the tempo and the tuning all come out of it. Anything the\n"+
				"file does not carry, such as which take you are playing, is not asked for.")
	}
	if m.tracks != nil {
		return m.viewTracks()
	}
	if len(m.songs) == 0 {
		return m.viewEmpty()
	}

	list := m.filtered()
	lines := []string{"", m.sectionHead("LIBRARY", m.libraryCount(list)), ""}

	if m.asking == askingFilter {
		lines = append(lines, styleAccent.Render("  /")+m.input.View(), "")
	} else if m.filter != "" {
		lines = append(lines, "  "+styleFaint.Render("/")+styleSubtle.Render(m.filter)+
			styleFaint.Render("   h clears it"), "")
	}

	if len(list) == 0 {
		lines = append(lines, "  "+styleFaint.Render("nothing matches that"))
	}

	for index, item := range list {
		lines = append(lines, m.songRow(item, index == m.pick))
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// libraryCount says how much of the library is on screen, and only mentions
// the filter when one is on.
func (m *Model) libraryCount(list []*song.Song) string {
	if m.filter == "" {
		return plural(len(m.songs), "song")
	}
	return fmt.Sprintf("%d of %s", len(list), plural(len(m.songs), "song"))
}

func (m *Model) songRow(item *song.Song, selected bool) string {
	mark := "   "
	title := styleInk.Render(item.Title)
	if selected {
		mark = styleAccent.Render(" ▎ ")
		title = styleHeading.Render(item.Title)
	}

	// the track name matters more than the artist here: a file has one artist
	// and five tracks, and the one on screen is the one being practised
	// a text tab has no tempo to print, and printing the one it was spaced
	// with would be claiming a rhythm the source never carried
	tempo := styleFaint.Render(fmt.Sprintf("♩%.0f", item.Tempo))
	if item.Untimed {
		tempo = styleFaint.Render("  text")
	}

	right := tempo + "  " + styleSubtle.Render(fmt.Sprintf("%3d measures", len(item.Measures)))

	middle := item.Artist
	if item.Track != "" {
		if middle != "" {
			middle += "  ·  "
		}
		middle += item.Track
	}

	left := mark + title
	if middle != "" {
		left += styleFaint.Render("   " + middle)
	}

	return pad(truncate(left, m.width-lipgloss.Width(right)-2), right, m.width)
}

func (m *Model) viewEmpty() string {
	body := []string{
		"",
		styleHeading.Render("  Nothing imported yet"),
		"",
		styleSubtle.Render("  A song comes from a guitar pro file, which is the only kind of tab"),
		styleSubtle.Render("  that carries its own tempo. Press " + styleAccent.Render("i") + styleSubtle.Render(" and give it a path.")),
		"",
		styleFaint.Render("  Songs are read from " + m.cfg.Library),
	}
	return strings.Join(body, "\n") + blank(m.space()-len(body))
}

func (m *Model) viewTracks() string {
	lines := []string{"", m.sectionHead("TRACKS IN THIS FILE", ""), ""}

	for index, track := range m.tracks {
		mark := "   "
		name := styleInk.Render(track.Name)
		if index == m.track {
			mark = styleAccent.Render(" ▎ ")
			name = styleHeading.Render(track.Name)
		}

		note := styleSubtle.Render(fmt.Sprintf("%d strings · %d measures", track.Strings, track.Measures))
		if !track.Playable {
			note = styleFaint.Render("drums, no tab to draw")
			name = styleFaint.Render(track.Name)
		}

		lines = append(lines, pad(mark+name, note, m.width))
	}

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// viewAsk is the one text field of the app, with room for a sentence saying
// what it wants. A prompt with no explanation is how people end up typing the
// wrong kind of path.
func (m *Model) viewAsk(title, explain string) string {
	lines := []string{"", "  " + styleHeading.Render(title), ""}
	for _, line := range strings.Split(explain, "\n") {
		lines = append(lines, "  "+styleSubtle.Render(line))
	}
	lines = append(lines, "", styleAccent.Render("  ▸")+m.input.View())

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}

// plural keeps the count and its noun agreeing, since a library of one is the
// state the app opens in the first time it is used.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// sectionHead is the small caps title every screen opens with.
func (m *Model) sectionHead(title, right string) string {
	left := "  " + styleAccent.Render(title)
	if right == "" {
		return left
	}
	return pad(left, styleFaint.Render(right), m.width)
}
