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

	// EventWorkerGone is written here and not by python, which by then is the
	// thing that is gone
	EventWorkerGone = "worker_gone"

	// EventScriptLog is a one shot script writing on stderr, and
	// EventScriptGone is one that could not be started at all. They are not
	// EventLog because the two mouths are told apart by the name of the event:
	// a log of the live worker's has to go back to the live worker
	EventScriptLog  = "script_log"
	EventScriptGone = "script_gone"

	EventListening   = "listening"
	EventListenError = "listen_error"

	// EventListenWaiting is an input that is not there yet and is being waited
	// for, which is not a failure: the worker opens it as soon as it is back
	EventListenWaiting = "listen_waiting"

	EventStopped      = "stopped"
	EventAudioWarning = "audio_warning"

	EventTracks      = "tracks"
	EventImported    = "imported"
	EventImportError = "import_error"

	EventSpotifyLog       = "spotify_log"
	EventSpotifyReady     = "spotify_ready"
	EventSpotifyPlaylists = "spotify_playlists"
	EventSpotifyTracks    = "spotify_tracks"
	EventSpotifyError     = "spotify_error"
)

// Command is what goes into the stdin of the live worker.
type Command struct {
	Action string `json:"action"`
	Device int    `json:"device,omitempty"`
	Rate   int    `json:"rate,omitempty"`

	// Source is the input to read, by the name the sound server gives it
	Source string `json:"source,omitempty"`

	// Card is what that input is plugged into, which a profile change does not
	// rename. It is what finds the input again when the name has changed
	Card string `json:"card,omitempty"`
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
	if _, err := os.Stat(".venv/bin/python"); err == nil {
		return ".venv/bin/python"
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
