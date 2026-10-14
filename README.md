# fretdeck

A terminal app for practicing guitar. It displays the tab of a song, listens to the audio input from your sound card, and verifies if the played note matches the tab.

## Features

- **Practice**: The cursor holds on a note until you play it right, then moves on. There is no clock: `h` and `l` walk a note at a time, `[` and `]` a measure at a time.
- **Looping**: Press `r` on the practice screen, use `Space` to select measures, and loop specific sections.
- **Metronome**: A screen of its own with the beat and the bar on it, counting out of the speakers while the microphone stays open, because the tone it counts with is one no string is ever tuned to.
- **Search & Download**: Queries Ultimate Guitar or Cifra Club, whichever the config screen names, directly from the terminal. Press `Enter` to download and save the tab as JSON in your local library. The tuning comes with it.
- **Difficulty Ratings**: Whatever the site the search went to says about a transcription, in its own words, and nothing at all where it says nothing.
- **Spotify Integration**: Import playlists or liked songs, and search for a tab of one without leaving the app.
- **Tuner**: The last section of the config screen, with the deviation drawn as a needle rather than a number, so it is clear which way the peg turns.
- **Recent Songs**: Opens your most recently played songs by default.

## Installation

### Requirements

- Go 1.26 or newer
- Python 3.9 or newer
- PortAudio (required by the `sounddevice` Python library)

On Fedora, install PortAudio via:
```sh
sudo dnf install portaudio
```

### Building

```sh
make install-python
make build
./fretdeck
```

If the audio libraries live in a virtualenv, point the app at its interpreter with `FRETDECK_PYTHON=~/venv/bin/python ./fretdeck`. While working on the source, `make run` opens the interface without building first and passes flags on: `make run ARGS='-song ~/fretdeck/songs/riff.json -device 3'`.

## First run

The app asks two things, one screen each: which instrument is plugged in, and which input to listen on. `j` and `k` move, `enter` keeps the answer, `h` goes back a step and `esc` leaves both for later. An answer that was kept is not asked about again, and both can be changed later on the config screen.

The instrument is the guitar, a seven string, the bass or a five string bass. A song carries its own tuning, so the answer is only used where there is no song: the tuner, and telling a bass tab from a guitar one in a list of search results.

The input is kept by the name the sound server gives it, not by index, because PortAudio renumbers when something is plugged in. When the input that was kept is gone the app says so and opens the config screen instead of falling back to the system default; plug the interface back in and press `r` to find it again.

## Keys

Vim, everywhere. `j` down, `k` up, `h` back or left, `l` into a list, `gg` and `G` for the two ends of one. `H` and `L` walk the screens. `enter` is the key that decides something: reading a tab in and opening a song are both it. `i` puts the cursor in the search field, and `?` puts the whole map on the screen.

There are three modes and `esc` is the way back from any of them: **normal**, **insert** (the cursor in the search field) and **repeat** (the practice screen picking measures to loop, which paints the app blue while it is on).

The mouse selects a row, opens a screen from the button bar and scrolls the list under the pointer. A click selects and does not open. It can be turned off on the config screen, since a terminal reporting the mouse stops selecting text with it.

## Where the songs come from

Two sites are read and the config screen picks one. The answer is kept.

**Ultimate Guitar** has a dozen transcriptions of a song, each with a version number and a rating, and it says whether one is a guitar tab, a bass tab or a chord sheet before it is opened. A search groups them: a song is one row, the best liked of its versions, and `enter` opens the rest out under it. Best liked is not the highest rating, since a rating stands on the votes under it.

**Cifra Club** is Brazilian and has one cifra a song, with no version and no rating, and it does not say what is in it: a cifra is chord names over lyrics with the tab blocks written in among them, so some songs there have frets to follow and some have none at all. The preview under the list is what answers that, and a page with no tab line in it is refused rather than read in as a song of nothing. The tuning and the capo are beside the text there too.

A song already in the library is read by the site it came from, whichever site is in force now, so changing the answer does not put the list out of reach.

## Getting a song in

Press `i` on the screen the app opens on, type an artist and a song, and `enter` on a result reads the tab out of the page and writes a `.json` into `~/fretdeck/songs`. Nothing is downloaded through a browser. `enter` on a song that is already there opens it for practice instead of reading it in again.

The bottom half of the screen is the tab of the row the cursor is on, so a transcription can be read before it is kept.

## The metronome

The metronome screen is the beat, how many beats are in the bar, and the switch between the two of them and silence. `space` starts the count and stops it from any row, `h` and `l` are less and more of the row the cursor is on, and `[` and `]` move the beat by ten. It keeps counting on the other screens, which is what practising to it is, and the numbers are kept between runs while the count itself is not.

It is the one thing here that makes a sound, and the tone is the whole of what lets it. It sits half a semitone off the grid a fretboard is written on and above the highest note the detector names, so what it can be mistaken for are pitches no string is ever tuned to, and the tracker stays deaf to exactly those for as long as the click is sounding. Nothing is drawn beating on the screen: a second count would disagree with the one being heard inside a bar, and the wrong one is the one nobody can tell from the right one.

## Spotify

The spotify screen logs in with a username and a password and lists the playlists on the account, plus the liked songs. Opening one lists its tracks in the order the account keeps them, and `enter` on a track searches the tab site for it. Nothing is streamed and nothing is played: the tabs are what is being looked for.

## Notes

The metronome is the one thing here that makes a sound, and it pays for it with a tone the detector cannot mistake for a string and a tracker deaf to that tone while it sounds. Nothing else does: there is no playback of a song, since a note out of the speakers would be heard as a note that was played.

A chord counts as played when any of its notes is heard, since the detector reports one pitch at a time. The low strings are allowed to read an octave high for the same reason: the fundamental of a wound string is quieter than its second harmonic, and failing that would fail somebody playing correctly.
