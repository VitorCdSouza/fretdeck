package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is what survives between runs. It lives in os.UserConfigDir, out of
// the repository, and none of it is required: a first run works with defaults.
type Config struct {
	// Device is the portaudio index of the input. Negative means the one the
	// system calls default, which is right until somebody plugs in an interface
	Device int `json:"device"`

	// Rate is the sample rate to open the device at. It has to be one the
	// device accepts, so it is stored next to the device it was chosen for
	Rate int `json:"rate"`

	// Library is where the imported songs live
	Library string `json:"library"`

	// Speed is the tempo multiplier the practice screen opens with
	Speed float64 `json:"speed"`

	// Downloads is watched after a tab is opened in the browser, so the file
	// lands in the library without anybody having to type where it went
	Downloads string `json:"downloads"`
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
	library, downloads := "songs", "."
	if home, err := os.UserHomeDir(); err == nil {
		library = filepath.Join(home, "fretdeck", "songs")
		downloads = filepath.Join(home, "Downloads")
	}
	return Config{Device: -1, Rate: 44100, Library: library, Speed: 1, Downloads: downloads}
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
	if loaded.Library == "" {
		loaded.Library = defaults().Library
	}
	if loaded.Downloads == "" {
		loaded.Downloads = defaults().Downloads
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
