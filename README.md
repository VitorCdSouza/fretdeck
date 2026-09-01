# fretdeck

A terminal app for practising guitar. It shows the tab of a song, listens to
what you play through the sound card, and tells you whether the note that came
out is the one on the screen.

```
  Sweet Child O Mine  ·  Guitar 1                          wait   measure 3   94%

      ▾
     3                    4                    5
  e  │────────────────────│────────────────────│───────────────
  B  │────────────────────│────────────────────│───────────────
  G  │──────────14────────│────────────────────│───────12──────
  D  │━━12━━━━━━━━━━━━━━━━│━━━━━━━━━━━14━━━━━━━│━━━━━━━━━━━━━━━
  A  │━━━━━━━━━━━━━━━━━━━━│━━━━━━━━━━━━━━━━━━━━│━━━━━━━━━━━━━━━
  E  │━━━━━━━━━━━━━━━━━━━━│━━━━━━━━━━━━━━━━━━━━│━━━━━━━━━━━━━━━

  play  D4   string 4 fret 12                              heard  D4  ✓

      1       3       5       7       9          12
  e  ║───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼
  B  ║───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼
  G  ║───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼
  D  ║━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━●━┼
  A  ║━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼
  E  ║━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼━━━┼
              ·       ·       ·       ·           :
```

## What it does

- **Follows a tab** while you play, in two modes. **Wait** holds on a note until
  you get it right, which is what learning a passage looks like. **Tempo** runs
  the song at its own speed, at 25% to 150% of it, and marks every note against
  the instant it was written for.
- **Or draws the same song as a highway**, six lanes with the notes coming at
  you and a line at the bottom that is now. `v` swaps between the two: the
  highway to play in time, the tab to learn a passage by heart.
- **Reads a plain text tab**, the kind most of the internet is written in, as
  well as a Guitar Pro file.
- **Searches songsterr** for a song, says how hard they call the guitar part,
  and marks the ones already in your library.
- **Reads a Spotify playlist**, or your liked songs, looks every track up and
  sorts the lot from easiest to hardest.
- **Draws the neck** under the tab, with the note to play as a dot on it. A tab
  says which fret; the neck says where the finger goes.
- **Tunes**, off the same detector, with the deviation as a needle instead of a
  number so it is clear which way the peg turns.
- **Marks a recording.** Record yourself playing the whole song and it reports
  the notes you missed, the ones you played instead, which measures were the
  trouble and how fast the take actually was.

## Installing

```bash
make install-python
make build
./fretdeck
```

Go 1.26 and Python 3.9 or newer. On Fedora the audio input also needs
PortAudio, which `sounddevice` binds to:

```bash
sudo dnf install portaudio
```

If the audio libraries live in a virtualenv, point the app at its interpreter:

```bash
FRETDECK_PYTHON=~/venv/bin/python ./fretdeck
```

## Moving around

Vim, everywhere. `j` down, `k` up, `h` back or left, `l` in or right, `gg` and
`G` for the two ends of a list. `H` and `L` change screen and so do the number
keys. `/` searches: the library on the library screen, songsterr on the search
one. `?` puts the whole map on the screen, and the four keys that matter most
are always on the bar at the bottom.

## Getting a song in

Songs come from Guitar Pro files, `.gp3` through `.gpx`. It is the only kind of
tab that carries its own tempo, and without a tempo there is nothing for the
tempo mode to mark against.

Press `i` on the library screen and give it a path. It reads the tracks in the
file, you pick one, and it writes a `.json` into `~/fretdeck/songs`. Nothing
else is needed after that: the imported file is the whole song.

### A plain text tab

`i` also takes a `.txt` of the six line tab everyone writes:

```
e|-----------------|-----------------|
B|-----------------|-------------12--|
G|-------------0---|-----------------|
D|---0---2---------|-----5-----------|
A|-2---------------|-----------------|
E|-----------------|-3---------------|
```

Save the tab block into a file and give it the path. It reads the frets, keeps
the bar lines as measures, believes a `Tuning: D A D G B e` line when there is
one, and knows a hammer on from a fret. A chord sheet is not mistaken for a
tab, and neither are the dashes in a line of lyrics.

**A text tab has no rhythm in it.** It carries the notes and their order and
nothing else: no durations, no tempo, not even which of two notes side by side
is the longer one. So a song imported this way is marked as text, and tempo
mode refuses it rather than marking you wrong for playing the rhythm the song
actually has. Wait mode and the highway both work on it, and between them that
is most of what learning a riff is.

### Finding one you do not have

The search screen asks songsterr. It answers which songs have a tab, which
instruments each was written for, and what they call the difficulty of the
guitar part.

**The file itself is not downloaded by the app, and cannot be.** Songsterr keeps
the tab data in a bucket that answers `AccessDenied` to anything without a
signed key, and exporting a Guitar Pro file is a paid feature of their site.
Anything that claimed to fetch it here would be a lie or a scraper that breaks
next week.

So `enter` on a result opens its page in your browser. Download the file there,
and the app takes over again: it watches your downloads folder for ten minutes,
and the moment a `.gp*` lands it reads the tracks and asks which one you want.

The app does not go and fetch tabs from anywhere by itself, songsterr included.
It searches, and it reads what you give it.

### From a Spotify playlist

Connect once on the setup screen with `s`. It opens Spotify in your browser and
keeps the session in the config folder. There is no app to register and no
client id to paste anywhere: the login is librespot, which is also what hands
out the access token.

After that, `s` on the search screen lists your playlists, with the liked songs
first. Open one and every track is looked up on songsterr, three at a time, and
the list sorts itself from easiest to hardest with the ones you already have
marked.

A track only counts as found when the artist and the title both match, ignoring
case, punctuation and the remaster note a streaming service leaves on a title.
Taking the first result instead would answer for a cover or a live version, and
report the difficulty of a transcription you were not asking about.

There is an exercise in `examples/` to try it with before importing anything:

```bash
mkdir -p ~/fretdeck/songs && cp examples/*.json ~/fretdeck/songs/
```

## The input

The setup screen lists every input PortAudio can see. A line in beats a
microphone by a distance: it hears the guitar and not the room, and it does not
hear the song you are playing along to.

Playing an electric straight into the line in of the sound card works. So does
an interface, and so does a microphone in front of an amp if it is the only
thing making noise.

## Reading a mark

The detector is monophonic: it names one pitch at a time, whichever is
loudest. That has two consequences worth knowing before trusting a score.

A chord counts as played when any of its notes is heard, because asking for six
at once from something that reports one would fail every chord in the song. And
a note an octave away from the written one counts as right, since the
fundamental of a wound string is quieter than its second harmonic and the
detector follows the harmonic often enough that refusing it would fail you for
playing correctly.

Single note lines are what it judges well. A strummed rhythm part it will
follow, but it is not marking your voicings.

## Layout

```
fretdeck/
├── cmd/fretdeck/       the entry point
├── internal/
│   ├── song/           the song model, the tab drawing and the text tab reader
│   ├── practice/       the modes, the clock and the scoring
│   ├── songsterr/      the search, and matching a streamed track to a tab
│   ├── bridge/         talking to the python side
│   ├── scripts/        the python side, embedded in the binary
│   ├── config/         what survives between runs
│   └── ui/             the screens
└── examples/           a song to start with
```

The interface is Go with Bubble Tea. The audio is Python, because the parts
that matter have no counterpart in Go: numpy for the arithmetic, PortAudio
through `sounddevice` for the input and PyGuitarPro for reading a tab. Spotify
is Python too, for the same reason: librespot has no Go port. The songsterr
search is Go, because it is one GET returning json and running a process for
every keystroke would be worse than the dependency it saves.

The two sides talk over one json object per line, and the scripts are embedded
in the binary and unpacked at startup, so the build runs from anywhere it is
copied to.
