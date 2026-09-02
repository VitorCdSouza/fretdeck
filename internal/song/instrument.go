package song

import "strings"

// Instrument is what is plugged in. Nothing else here knows: a song carries
// its own tuning and every screen reads it from there, so the answer is only
// needed where there is no song, which is the tuner, the neck and a search
// that answers for a guitar and a bass in the one list.
type Instrument struct {
	// Name is what the setup shows and what the config keeps
	Name string

	// Bass says which half of a catalogue a search should be read from
	Bass bool

	// Tuning is the open strings, thinnest first, the way a song carries them
	Tuning []String
}

// Instruments is what can be answered on the setup screen. The tuning of each
// is the standard one, which is what somebody who has not said otherwise is
// holding.
var Instruments = []Instrument{
	{Name: "guitar", Tuning: tuned(64, 59, 55, 50, 45, 40)},
	{Name: "guitar, seven string", Tuning: tuned(64, 59, 55, 50, 45, 40, 35)},
	{Name: "bass", Bass: true, Tuning: tuned(43, 38, 33, 28)},
	{Name: "bass, five string", Bass: true, Tuning: tuned(43, 38, 33, 28, 23)},
}

// Chosen is the instrument the config names, and the guitar when it names
// nothing or names one this build does not have.
func Chosen(name string) Instrument {
	for _, item := range Instruments {
		if item.Name == name {
			return item
		}
	}
	return Instruments[0]
}

// Written is the tuning the way a tab site writes it, thickest string first,
// which is the opposite of how the lines are drawn.
func (i Instrument) Written() string {
	names := make([]string, 0, len(i.Tuning))
	for index := len(i.Tuning) - 1; index >= 0; index-- {
		names = append(names, strings.TrimRight(NoteName(i.Tuning[index].Midi), "0123456789"))
	}

	return strings.Join(names, " ")
}

// tuned numbers the strings thinnest first, which is how guitar pro counts
// them and how a tab is drawn.
func tuned(midis ...int) []String {
	out := make([]String, len(midis))
	for index, midi := range midis {
		out[index] = String{Number: index + 1, Midi: midi}
	}
	return out
}
