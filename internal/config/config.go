package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is what survives between runs. It lives in os.UserConfigDir, out of
// the repository, and a first run has none of it: the setup asks for the two
// answers that cannot be guessed and keeps them.
type Config struct {
	// Device is the portaudio index of the input. Negative is what a first run
	// looks like, and it is what makes the setup ask which one to listen on
	Device int `json:"device"`

	// Source is the name the sound server gives the input, which survives a replug
	Source string `json:"source"`

	// Card is what the input is plugged into. The name of a node carries the
	// profile of the card in it, so switching a card between duplex and pro
	// audio renames it, and this is the half that stays
	Card string `json:"card"`

	// Instrument is what is plugged in, by the name the song package gives it.
	// Empty is the other half of a first run, asked before the input is
	Instrument string `json:"instrument"`

	// Rate is the sample rate to open the device at. It has to be one the
	// device accepts, so it is stored next to the device it was chosen for
	Rate int `json:"rate"`

	// Library is where the imported songs live
	Library string `json:"library"`

	// Speed is the tempo multiplier the practice screen opens with
	Speed float64 `json:"speed"`

	// Bpm is the beat the metronome opens on. It is only used where the song
	// has no tempo of its own to take one from, since a song that has one
	// scales that instead and the two would otherwise disagree
	Bpm float64 `json:"bpm"`

	// Click says whether the metronome is heard as well as seen. It is off
	// unless it was turned on: the input is open while it counts
	Click bool `json:"click"`

	// Level is how much of a transcription to ask for, by the name the song
	// package gives it. An empty one is the bottom of the ladder
	Level string `json:"level"`

	// Mouse says whether the app asks the terminal for mouse events. It is on
	// unless it was turned off, and turning it off gives text selection back
	Mouse bool `json:"mouse"`
}

// Answered is whether the first run has both of its answers. One that was
// kept is not asked about again, which is the whole point of keeping it.
func (c Config) Answered() bool {
	return c.Instrument != "" && c.Device >= 0
}

// Credentials is where the spotify login is kept. It is written by librespot
// and never read here, which is why it is only a path.
func Credentials() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "spotify.json"), nil
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, "fretdeck")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return dir, nil
}

func defaults() Config {
	library := "songs"
	if home, err := os.UserHomeDir(); err == nil {
		library = filepath.Join(home, "fretdeck", "songs")
	}
	return Config{
		Device:  -1,
		Rate:    44100,
		Library: library,
		Speed:   1,
		Bpm:     120,
		Level:   "full",
		Mouse:   true,
	}
}

func Load() Config {
	loaded := defaults()

	dir, err := Dir()
	if err != nil {
		return loaded
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return loaded
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return defaults()
	}

	if loaded.Rate <= 0 {
		loaded.Rate = 44100
	}
	if loaded.Speed <= 0 {
		loaded.Speed = 1
	}
	if loaded.Bpm <= 0 {
		loaded.Bpm = defaults().Bpm
	}
	if loaded.Level == "" {
		loaded.Level = defaults().Level
	}
	if loaded.Library == "" {
		loaded.Library = defaults().Library
	}
	return loaded
}

func (c Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)
}
