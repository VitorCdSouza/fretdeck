package song

// Technique is how a note is got at, when the source said so. It is read off
// the file and never invented: a source that carries no effects gives every
// note an empty one, which is a note picked plain.
type Technique string

const (
	Hammer   Technique = "hammer"
	Pull     Technique = "pull"
	Slide    Technique = "slide"
	Bend     Technique = "bend"
	Vibrato  Technique = "vibrato"
	Harmonic Technique = "harmonic"
	Tap      Technique = "tap"
	Palm     Technique = "palm"
)

// Level is how much of the transcription the song asks for. It is a ladder and
// not a switch, so the step between two levels is one hand learning one thing.
type Level int

const (
	// Plain is the notes and their order, which is all a text tab ever has
	Plain Level = iota

	// Basic adds what the fretting hand does on its own, without the other
	// hand striking the string again
	Basic

	// Full is everything the file carries
	Full
)

// Levels is the ladder in order, for a key that walks up it.
var Levels = []Level{Plain, Basic, Full}

func (l Level) String() string {
	switch l {
	case Basic:
		return "basic"
	case Full:
		return "full"
	}
	return "plain"
}

// ReadLevel turns the name kept in the config back into a level. Anything else
// is the bottom of the ladder, which asks for nothing that is not a note.
func ReadLevel(name string) Level {
	for _, level := range Levels {
		if level.String() == name {
			return level
		}
	}
	return Plain
}

// Level is the rung a technique sits on. One the app does not know is Full,
// since a technique it cannot name is not one it can call easy.
func (t Technique) Level() Level {
	switch t {
	case "":
		return Plain
	case Hammer, Pull, Slide:
		return Basic
	}
	return Full
}

// Mark is the letter written beside the fret, the way an ascii tab writes it.
func (t Technique) Mark() string {
	switch t {
	case Hammer:
		return "h"
	case Pull:
		return "p"
	case Slide:
		return "s"
	case Bend:
		return "b"
	case Vibrato:
		return "~"
	case Harmonic:
		return "*"
	case Tap:
		return "t"
	case Palm:
		return "m"
	}
	return ""
}

// marks is how a written tab spells each of them, for the parser to read back.
var marks = map[rune]Technique{
	'h':  Hammer,
	'p':  Pull,
	's':  Slide,
	'/':  Slide,
	'\\': Slide,
	'b':  Bend,
	'r':  Bend,
	'~':  Vibrato,
	'v':  Vibrato,
	'*':  Harmonic,
	'<':  Harmonic,
	't':  Tap,
	'm':  Palm,
}

// ReadMark is the technique a letter of a written tab stands for, and an empty
// one for a letter that is not a technique at all.
func ReadMark(r rune) Technique { return marks[r] }

// Hardest is the one of a set that decides what the note is worth. A note is
// on one rung of the ladder, so a bend with vibrato over it is a bend.
func Hardest(found []Technique) Technique {
	best := Technique("")
	for _, one := range found {
		if one.Level() > best.Level() {
			best = one
		}
	}
	return best
}
