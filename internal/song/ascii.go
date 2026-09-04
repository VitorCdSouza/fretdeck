package song

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Reading the plain text tab that most of the internet is written in.
//
// It carries the notes and the bar lines and nothing else: no duration, no
// tempo, not even which of two notes side by side is the longer one. So a song
// read from here is marked Untimed, and the modes that need a clock say so
// rather than marking somebody wrong for playing the real rhythm.

// untimedTempo is the tempo an untimed song reports. It is not a claim about
// the music. It only spaces the notes evenly so the highway has something to
// scroll and the tab has a width to draw.
const untimedTempo = 80.0

// label is what a tab line starts with: a note name, then the bar. Both halves
// are optional, since plenty of tabs are written with only one of them.
var label = regexp.MustCompile(`^[ \t]*([A-Ga-g][#b]?)?[ \t]*[|:]?`)

// techniques are the letters written between the frets. They say how to get
// from one note to the next, which this app has nothing to do with, but they
// have to be recognized or a hammer on would read as a fret.
const techniques = "hpbrsvtx/\\~^*()<>=+ "

// ParseASCII reads a text tab. The text may be the whole page somebody copied:
// everything that is not a run of tab lines is skipped.
func ParseASCII(text, title string) (*Song, error) {
	return ParseTuned(text, title, "")
}

// ParseTuned reads a text tab whose tuning was written down apart from it,
// which is how a tab site keeps it. The value is what such a site writes, the
// thickest string first, as in "E A D G B E", and an empty one falls back to
// reading the text.
func ParseTuned(text, title, tuning string) (*Song, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	parsed := &Song{
		Title:   title,
		Tempo:   untimedTempo,
		Untimed: true,
	}

	var events [][]Note
	measure := 1
	width := 0

	for index := 0; index < len(lines); {
		block, next := readBlock(lines, index)
		index = next
		if len(block) < 2 {
			continue
		}

		if width == 0 {
			width = len(block)
			parsed.Tuning = tuningFrom(width, tuning, text)
		}
		if len(block) != width {
			// a block with a different number of lines is a different
			// instrument pasted under the first one, or a stray line that
			// looked like a tab. either way it is not this song
			continue
		}

		read, bars := readColumns(block, parsed.Tuning, measure)
		events = append(events, read...)
		measure += bars
	}

	if len(events) == 0 {
		return nil, errors.New("no tab lines were found in that file")
	}

	parsed.Notes, parsed.Measures = layout(events)

	return parsed, nil
}

// readBlock takes the run of tab lines starting at index, and answers where the
// run ended. A blank line or a line of words ends it.
func readBlock(lines []string, index int) ([]string, int) {
	var block []string

	for ; index < len(lines); index++ {
		if !tabLine(lines[index]) {
			if len(block) > 0 {
				return block, index
			}
			continue
		}
		block = append(block, body(lines[index]))
	}

	return block, index
}

// tabLine answers whether a line is one string of a tab. The test is what the
// line is made of: a tab line is dashes with frets in it, and a line of lyrics
// is not, however many stray dashes it has.
func tabLine(line string) bool {
	text := body(line)
	if len(text) < 4 {
		return false
	}

	dashes := 0
	for _, r := range text {
		if r == '-' || r == '|' {
			dashes++
		}
	}

	// half the line has to be the string itself, or a row of chord names over
	// a lyric would qualify
	return dashes*2 >= len(text) && dashes > 0
}

// body is the string of the tab and nothing else: the label is taken off the
// front, and so is whatever was written after the last dash, since a tab line
// with a word beside it is still a tab line.
func body(line string) string {
	text := strip(line)

	end := 0
	for index, r := range text {
		switch {
		case r == '-' || r == '|':
			end = index + 1
		case r >= '0' && r <= '9', strings.ContainsRune(techniques, r):
		default:
			return text[:end]
		}
	}

	return text[:end]
}

func strip(line string) string {
	return strings.TrimRight(label.ReplaceAllString(line, ""), " \t")
}

// readColumns walks the block left to right. A column is one instant, and the
// bar lines it crosses are counted so the measures come from the tab and not
// from a guess.
func readColumns(block []string, tuning []String, measure int) ([][]Note, int) {
	longest := 0
	for _, line := range block {
		if len(line) > longest {
			longest = len(line)
		}
	}

	var events [][]Note
	bars := 0

	for column := 0; column < longest; column++ {
		if barAt(block, column) {
			bars++
			measure++
			continue
		}

		var event []Note
		for row, line := range block {
			fret, ok := fretAt(line, column)
			if !ok {
				continue
			}
			number := tuning[row].Number
			event = append(event, Note{
				Measure:   measure,
				String:    number,
				Fret:      fret,
				Midi:      tuning[row].Midi + fret,
				Technique: markAt(line, column),
			})
		}

		if len(event) > 0 {
			events = append(events, event)
		}
	}

	return events, bars
}

func barAt(block []string, column int) bool {
	for _, line := range block {
		if column >= len(line) || line[column] != '|' {
			return false
		}
	}
	return true
}

// fretAt reads the number starting at this column, and answers false when the
// column is the second digit of a fret that started before it. Without that a
// twelfth fret would be read as a first and a second.
func fretAt(line string, column int) (int, bool) {
	if column >= len(line) || !digit(line[column]) {
		return 0, false
	}
	if column > 0 && digit(line[column-1]) {
		return 0, false
	}

	end := column
	for end < len(line) && digit(line[end]) {
		end++
	}

	fret, err := strconv.Atoi(line[column:end])
	if err != nil || fret > 24 {
		return 0, false
	}

	return fret, true
}

func digit(b byte) bool { return b >= '0' && b <= '9' }

// markAt is the technique written on the fret that starts at this column.
//
// A letter between two frets belongs to the one it leads to, since that is the
// note it says how to reach: the 7 of 5h7 is hammered on and the 9 of 7b9 is
// bent up to. A letter with no fret after it trails the note it is written
// against, which is how vibrato is drawn.
func markAt(line string, column int) Technique {
	if column > 0 {
		if found := ReadMark(rune(line[column-1])); found != "" {
			return found
		}
	}

	end := column
	for end < len(line) && digit(line[end]) {
		end++
	}
	if end >= len(line) {
		return ""
	}
	if end+1 < len(line) && digit(line[end+1]) {
		return ""
	}

	return ReadMark(rune(line[end]))
}

// layout spaces the events evenly and writes the measures out. The spacing is
// not a rhythm and is not offered as one: it is what lets the tab have a width
// and the highway have something to scroll.
func layout(events [][]Note) ([]Note, []Measure) {
	perNote := 60.0 / untimedTempo

	var notes []Note
	var measures []Measure
	seen := 0

	for index, event := range events {
		for _, note := range event {
			note.Beat = float64(index)
			note.Time = float64(index) * perNote
			note.Dur = perNote
			notes = append(notes, note)
		}

		if number := event[0].Measure; number != seen {
			seen = number
			measures = append(measures, Measure{
				Index:     number,
				Beat:      float64(index),
				Time:      float64(index) * perNote,
				Tempo:     untimedTempo,
				Signature: [2]int{4, 4},
			})
		}
	}

	return notes, measures
}

// tuningFrom takes the first of three answers that fits: the tuning the source
// carried alongside the tab, the one the text says in words, and the standard
// tuning of an instrument with that many strings.
func tuningFrom(count int, value, text string) []String {
	if named := stringsFor(strings.Fields(value), count); named != nil {
		return named
	}
	return tuningFor(count, text)
}

// tuningFor answers what the strings are tuned to. A tab that says so in words
// is believed; otherwise it is the standard tuning of an instrument with that
// many strings, which is right nearly every time and visible on the screen when
// it is not.
func tuningFor(count int, text string) []String {
	if named := namedTuning(count, text); named != nil {
		return named
	}

	// standard six string, and the seven and the four are the same shape with
	// a string added under it or two taken off the top
	standard := []int{64, 59, 55, 50, 45, 40, 35}

	switch {
	case count == 4:
		standard = []int{43, 38, 33, 28}
	case count > len(standard):
		count = len(standard)
	}

	tuning := make([]String, count)
	for index := 0; index < count; index++ {
		tuning[index] = String{Number: index + 1, Midi: standard[index]}
	}

	return tuning
}

var tuningLine = regexp.MustCompile(`(?i)tuning\s*[:=]?\s*([A-Ga-g][#b]?( +[A-Ga-g][#b]?)+)`)

// namedTuning reads a line like "Tuning: D A D G A D", which is how a tab that
// is not in standard tuning says so.
func namedTuning(count int, text string) []String {
	match := tuningLine.FindStringSubmatch(text)
	if match == nil {
		return nil
	}

	return stringsFor(strings.Fields(match[1]), count)
}

// stringsFor turns the letters of a written tuning into strings. They are
// written from the thickest, which is the opposite of how the lines are drawn.
func stringsFor(names []string, count int) []String {
	if count == 0 || len(names) != count {
		return nil
	}

	tuning := make([]String, count)
	for index, name := range names {
		midi, ok := openMidi(name, count-index)
		if !ok {
			return nil
		}
		tuning[count-1-index] = String{Number: count - index, Midi: midi}
	}

	return tuning
}

var pitchClass = map[string]int{
	"c": 0, "c#": 1, "db": 1, "d": 2, "d#": 3, "eb": 3, "e": 4, "f": 5,
	"f#": 6, "gb": 6, "g": 7, "g#": 8, "ab": 8, "a": 9, "a#": 10, "bb": 10, "b": 11,
}

// openMidi turns a letter into a pitch by putting it in the octave that string
// of a guitar lives in. A tuning is written without octaves, and the sixth
// string is never the same E as the first.
func openMidi(name string, number int) (int, bool) {
	class, ok := pitchClass[strings.ToLower(name)]
	if !ok {
		return 0, false
	}

	// the octave each string sits in, thickest last, taken from standard
	// tuning and shifted with the letter
	octaves := map[int]int{1: 4, 2: 3, 3: 3, 4: 3, 5: 2, 6: 2, 7: 1}
	octave, ok := octaves[number]
	if !ok {
		octave = 2
	}

	return (octave+1)*12 + class, true
}

// Write saves a song into the library and answers where it went. The name
// comes from the title, since a text tab has no file of its own to be named
// after the way a guitar pro import does.
func Write(dir string, parsed *Song) (string, error) {
	return WriteAs(dir, parsed, parsed.Title)
}

// WriteAs saves a song under a name of the caller's choosing. A tab pulled off
// a tab site needs one: a song has a dozen transcriptions there, and naming
// them all after the title would leave one file.
func WriteAs(dir string, parsed *Song, name string) (string, error) {
	data, err := json.MarshalIndent(parsed, "", " ")
	if err != nil {
		return "", err
	}

	file := slug(name)
	if file == "" {
		file = "untitled"
	}

	path := filepath.Join(dir, file+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}

	parsed.Path = path
	return path, nil
}

func slug(title string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}

	name := out.String()
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return strings.Trim(name, "-")
}

// LoadASCII reads a text tab off disk. The title is the file name, because a
// text tab rarely says its own name in a way worth trusting.
func LoadASCII(path string) (*Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	base := filepath.Base(path)
	title := strings.TrimSuffix(base, filepath.Ext(base))

	return ParseASCII(string(data), title)
}

// IsText answers whether a path is a text tab rather than a guitar pro file.
func IsText(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".tab", ".text", "":
		return true
	}
	return false
}
