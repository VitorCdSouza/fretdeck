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

## Getting a song in

Songs come from Guitar Pro files, `.gp3` through `.gpx`. It is the only kind of
tab that carries its own tempo, and without a tempo there is nothing for the
tempo mode to mark against.

Press `i` on the library screen and give it a path. It reads the tracks in the
file, you pick one, and it writes a `.json` into `~/fretdeck/songs`. Nothing
else is needed after that: the imported file is the whole song.

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
│   ├── song/           the song model and the tab drawing
│   ├── practice/       the modes, the clock and the scoring
│   ├── bridge/         talking to the python side
│   ├── scripts/        the python side, embedded in the binary
│   ├── config/         what survives between runs
│   └── ui/             the screens
└── examples/           a song to start with
```

The interface is Go with Bubble Tea. The audio is Python, because the parts
that matter have no counterpart in Go: numpy for the arithmetic, PortAudio
through `sounddevice` for the input and PyGuitarPro for reading a tab. The two
talk over one json object per line, and the scripts are embedded in the binary
and unpacked at startup, so the build runs from anywhere it is copied to.
