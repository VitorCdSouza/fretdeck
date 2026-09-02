package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// The mouse is the second way in and not a replacement: clicking a button on
// the navigation line opens that screen, clicking a row selects it, and the wheel
// scrolls the list under the pointer. Every one of them is a key as well, and
// the keys stay the documented way around.
//
// It costs something. A terminal asked for mouse events stops selecting text
// with the mouse, which is how anybody copies a path or an error off the
// screen, so the config screen has the switch that turns it off.

func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	// a question on the screen is answered with the keyboard, not by a click
	if m.input.Focused() || m.removing {
		return nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.move(-1)
		return nil
	case tea.MouseButtonWheelDown:
		m.move(1)
		return nil
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}

	// the help is over the whole screen, so a click on it only closes it
	if m.helping {
		m.helping = false
		return nil
	}

	// the whole button answers, border and all, and not the name alone
	if msg.Y >= tabTop && msg.Y < tabTop+tabRows {
		if which, ok := m.screenAt(msg.X); ok {
			return m.goTo(which)
		}
		return nil
	}

	if row, ok := m.rowAt(msg.Y); ok {
		return m.point(row)
	}

	return nil
}

// rowAt is the row of the list drawn on a line of the screen.
func (m *Model) rowAt(y int) (int, bool) {
	for _, run := range m.clicks {
		if y >= run.top && y < run.top+run.count {
			return run.first + y - run.top, true
		}
	}
	return 0, false
}

// point puts the cursor of the screen on a row, which is all a click does. It
// selects and does not open: the key that opens is the one on the bar.
//
// The login is the one thing it presses, because the spotify screen with no
// session on it is a single button and there is nothing there to select.
func (m *Model) point(row int) tea.Cmd {
	switch m.screen {
	case screenMusic:
		m.found = clamp(row, len(m.results))
	case screenConfig:
		m.configRow = clamp(row, m.configCount())
	case screenSpotify:
		if !m.linked {
			return m.login()
		}
		m.picked = clamp(row, m.spotifyRows())
	}
	return nil
}
