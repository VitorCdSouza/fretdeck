package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/ui"
)

// The flags are for working on the app, not for using it: everything they say
// is on a screen already, and saying it here saves opening those screens on
// every run of `make run`.
func main() {
	options := ui.Options{Device: -1}
	flag.StringVar(&options.Song, "song", "", "open a song from the library and go straight to it")
	flag.IntVar(&options.Device, "device", -1, "listen on this input for one run, without keeping it")
	flag.Parse()

	// the python side is checked before the screen is taken over, or a missing
	// library would show up as an empty interface with an error in a corner
	if err := bridge.Dependencies(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, "\ninstall them with:\n  pip install -r requirements.txt")
		os.Exit(1)
	}

	model := ui.NewWith(options)
	defer model.Close()

	screen := []tea.ProgramOption{tea.WithAltScreen()}
	if model.Mouse() {
		screen = append(screen, tea.WithMouseCellMotion())
	}

	if _, err := tea.NewProgram(model, screen...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
