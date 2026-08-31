// Package bridge runs the python side and turns its output into events.
//
// One json object per line in both directions. Anything that is not json is a
// traceback or a warning from a library and becomes a log event, because
// killing the interface over a line printed by portaudio would be worse than
// the line itself.
package bridge

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/VitorCdSouza/fretdeck/internal/scripts"
)

// Event is what the python side writes on stdout.
type Event struct {
	Event   string          `json:"event"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Event names. Every screen ties the ones it cares about to a transition.
const (
	EventLog     = "log"
	EventError   = "error"
	EventDevices = "devices"
	EventNote    = "note"
	EventLevel   = "level"

	EventListening    = "listening"
	EventListenError  = "listen_error"
	EventStopped      = "stopped"
	EventAudioWarning = "audio_warning"

	EventTracks      = "tracks"
	EventImported    = "imported"
	EventImportError = "import_error"

	EventProgress     = "progress"
	EventReport       = "report"
	EventAnalyzeError = "analyze_error"
)

// Command is what goes into the stdin of the live worker.
type Command struct {
	Action string `json:"action"`
	Device int    `json:"device,omitempty"`
	Rate   int    `json:"rate,omitempty"`
}

// Decode reads the data of an event into v.
func (e Event) Decode(v any) error {
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, v)
}

// python is the interpreter to run. It is overridable because a machine with a
// virtualenv for the audio libraries has them nowhere near the system python.
func python() string {
	if chosen := os.Getenv("FRETDECK_PYTHON"); chosen != "" {
		return chosen
	}
	return "python3"
}

// Dependencies answers whether the python side can run at all, and says which
// import failed when it cannot.
func Dependencies() error {
	cmd := exec.Command(python(), "-c", "import numpy, sounddevice, soundfile, guitarpro")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	return &MissingError{Output: string(output)}
}

type MissingError struct {
	Output string
}

// Error keeps the last line of the traceback and drops the rest. The frames
// are about a one line import and say nothing the name of the module does not.
func (e *MissingError) Error() string {
	lines := strings.Split(strings.TrimSpace(e.Output), "\n")
	last := lines[len(lines)-1]
	if last == "" {
		last = "the interpreter said nothing"
	}
	return "the python side cannot start: " + strings.TrimSpace(last)
}

// dir is unpacked once and reused, since the hash makes the path stable.
var dir string

func scriptPath(name string) (string, error) {
	if dir == "" {
		unpacked, err := scripts.Unpack()
		if err != nil {
			return "", err
		}
		dir = unpacked
	}
	return dir + string(os.PathSeparator) + name, nil
}
