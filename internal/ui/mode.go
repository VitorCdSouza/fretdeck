package ui

// The app has modes the way vim does, and esc is the way back to the normal
// one from any of them. Insert is the text field open and taking letters,
// repeat is the practice screen picking the measures to loop over, and normal
// is everything else, which is where the keys mean what the bar says they do.
//
// A mode is worth having only if it is visible. The palette says which one is
// on, the bar at the bottom says what its keys do, and nothing else changes
// under somebody who never presses i or r.
type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeRepeat
)

func (m mode) String() string {
	switch m {
	case modeInsert:
		return "insert"
	case modeRepeat:
		return "repeat"
	}
	return "normal"
}

// setMode is the one way in and out of a mode, since the palette follows it
// and a mode left behind somewhere else would leave the whole app blue.
func (m *Model) setMode(next mode) {
	if m.mode == next {
		return
	}
	m.mode = next
	repaint(next == modeRepeat)
}

// normal is what esc does. The picked measures are left alone: they are what
// is being practised, and leaving the mode is only putting the keys back.
func (m *Model) normal() {
	if m.input.Focused() {
		m.closeInput()
		return
	}
	m.setMode(modeNormal)
	m.status = ""
}
