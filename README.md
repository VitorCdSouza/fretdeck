# fretdeck

A terminal app for practicing guitar. It displays the tab of a song, listens to the audio input from your sound card, and verifies if the played note matches the tab.

## Features

- **Practice Modes**: 
  - **Wait**: Pauses the progression until you play the correct note. Ideal for learning new passages.
  - **Tempo**: Plays the song at a set speed (25% to 150%) and scores notes based on exact timing.
- **Visuals**: Toggle between a standard tab view and a vertical highway (`v`). Includes a fretboard view indicating exact finger placement.
- **Search & Download**: Queries Ultimate Guitar directly from the terminal. Press `Enter` to download and save the tab as JSON in your local library.
- **Difficulty Ratings**: Fetches track difficulty from Songsterr automatically for search results and playlists.
- **Spotify Integration**: Import playlists or liked songs. Automatically sorts tracks by difficulty for your selected instrument.
- **Looping**: Press `r` on the practice screen, use `Space` to select measures, and loop specific sections.
- **Tuner**: Built-in tuner with visual needle indication.
- **Recent Songs**: Opens your most recently played songs by default.

## Installation

### Requirements

- Go 1.26 or newer
- Python 3.9 or newer
- PortAudio (required by the `sounddevice` Python library)

On Fedora, install PortAudio via:
```sh
sudo dnf install portaudio
