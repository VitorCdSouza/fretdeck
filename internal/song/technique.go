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
