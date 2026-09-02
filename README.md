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

- **Follows a tab** while you play. **Wait** mode holds on a note until you get
  it right, which is what learning a passage looks like. **Tempo** mode runs the
  song at its own speed, at 25% to 150% of it, and marks every note against the
  instant it was written for, which needs a tab that carries a rhythm.
- **Or draws the same song as a highway**, six lanes with the notes coming at
  you and a line at the bottom that is now. `v` swaps between the two: the
  highway to play in time, the tab to learn a passage by heart.
- **Finds the song and reads it in.** The search is Ultimate Guitar, the text of
  the tab is in the page, and enter on a version writes it into the library
  without leaving the terminal. The tuning comes with it.
- **Says how hard it is.** Songsterr answers that beside every row, looked up
  behind the list while you read it.
- **Opens on what you played last.** With nothing typed the search screen is the
  list of songs you have been working on, and enter on one opens it again.
- **Reads a Spotify playlist**, or your liked songs, on a screen of its own:
  log in once, pick a playlist, and every track is looked up and the lot sorted
  from easiest to hardest on the instrument you play.
- **Repeats the passage you pick.** `r` on the practice screen and `space` over
  a measure, as many as you like, and the song loops those bars until you let
  them go.
- **Draws the neck** under the tab, with the note to play as a dot on it. A tab
  says which fret; the neck says where the finger goes.
- **Tunes**, off the same detector, with the deviation as a needle instead of a
  number so it is clear which way the peg turns.

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

While working on it, `make run` opens the interface straight from the source
and nothing has to be built first. It passes flags on:

```bash
make run ARGS='-song ~/fretdeck/songs/riff.json -device 3'
```

Both flags are for that and only that. `-song` opens the app on a song instead
of the search screen, and `-device` listens on an input for one run without
keeping it, because everything they say is already a screen away.

## The first run

The first time it opens it asks two things as two steps, in this order: what
instrument is plugged in, and which input to listen on. One question a screen,
`j` and `k` to move and `enter` to keep the answer and go on. `h` goes back a
step, `esc` leaves both for later, and the app opens on the search screen the
moment the second one is answered. An answer that was kept is not asked about
again, so every run after that opens straight on the search.

The instrument is the guitar, a seven string, the bass or a five string bass.
A song carries its own tuning and every screen reads it from there, so the
answer is only used where there is no song: the tuner and the neck open on the
right strings with nothing loaded, and a search says which of the tabs it found
are for the instrument being played rather than for the other one.

Both answers can be changed later on the config screen, which lists them one
under the other along with the mouse switch.

### When the input goes missing

A PortAudio index is not stable. Plugging an interface in after the app started
renumbers everything, and so does unplugging one before it, which is why the
answer is kept by the name the sound server gives the input and not by number.

When the input that was kept is not on the list any more, the app says so and
opens the config screen rather than quietly falling back to the system default,
which is how you end up practising into a laptop microphone without knowing it.
The saved answer is left where it is: plug the interface back in, press `r`,
and it is found where it was.

## Moving around

Vim, everywhere. `j` down, `k` up, `h` back or left, `l` into a list, `gg` and
`G` for the two ends of a list. `H` and `L`, the two of them shifted, are the
whole of how the screens are walked. `enter` is the key that decides something:
reading a tab in and opening a song are both it, and `l` does neither. `i` puts
the cursor in the search field. `?` puts the whole map on the screen, and the
keys that matter most are on the bar at the bottom, written as `[i] search
song`: as many of them as the line has room for, and `[?] keys` last, since the
rest of them are behind it.

The screens are a row of buttons under the name, each with a bar under it and
a thicker one under the screen that is open, with `[H]` against the left of the
strip and `[L]` against the right, each on the side it walks towards. The whole
of it sits in the middle of the line.

There are four buttons and five screens. A song being practised is not one of
them: it is the music screen with a song open on it, so it lights that button,
it is reached by opening a song and `backspace` or `esc` is the way back to the
list.

### Modes

Vim has modes and so does this, three of them, and `esc` is the way back from
any of them to the plain one.

- **normal** is where the keys mean what the bar at the bottom says they do.
- **insert** is the one text field taking letters. The music screen has a search
  section at the top with the field always drawn in it, and `i` is what puts the
  cursor there. `enter` searches, `esc` leaves the field where it is.
- **repeat** is the practice screen picking a passage to loop over. `r` turns it
  on and the whole app goes blue while it is, so a mode that changes what the
  keys do is seen and not remembered.

### The mouse

Clicking a name on that line opens the screen, clicking a row selects it, and
the wheel scrolls the list under the pointer. A click selects and does not
open: the key that opens is the one on the bar at the bottom. The login button
on the spotify screen is the one exception, since a screen with a single button
on it has nothing to select.

It costs something, and the cost is why there is a switch. A terminal that is
asked to report the mouse stops selecting text with it, which is how anybody
copies a path or an error off the screen. Most terminals still select while
shift is held; the config screen turns the whole thing off for the ones that do
not, and it stays off.

## Getting a song in

There is one way in and it is the screen the app opens on. Press `i`, type an
artist and a song, and Ultimate Guitar answers with every version of it people
have written. `enter` on one reads the text out of the page and writes a `.json`
into `~/fretdeck/songs`. Nothing is downloaded through a browser and nothing has
to be found in a downloads folder: the tab is in the page.

A row says which version it is, how many people rated it and how well, and what
songsterr calls the difficulty of the song. `enter` on a song that is already
here opens it for practice instead of reading it in again.

The bottom half of the screen is the tab itself. Rest the cursor on a version
for a moment and the first lines of it are drawn there, which is the thing that
actually answers whether it is the one worth taking. The page is read once and
kept, so walking back up the list costs nothing and `enter` on a version that
has been looked at does not read it twice.

Four of the five things that site calls a tab have no frets in them. A chord
sheet is chord names over lyrics, and the pro and power files are binaries
behind a subscription that the page does not serve. They stay on the list
because knowing a song is only up as chords is an answer too, and `enter` on
one says so rather than writing an empty song.

### The difficulty beside a row

That number is songsterr's, and it is about their transcription and not the one
about to be read in. It answers how hard the song is, which is the question
somebody is asking on a search screen.

It is looked up per song and not per row: twenty versions of one song are one
request. Three at a time, because a playlist is hundreds of songs and firing
hundreds of requests at once is how the search stops answering at all. A song
whose artist and title do not both match on songsterr has no number beside it,
and that blank means not found, not easy.

### What you played last

With nothing typed the screen is what has been read in and played, newest
first. A song that is on disk says so, and `d` on one asks before it removes
the file and the line together. A song whose file has gone keeps its line, since
the page it came from is still the answer to finding it again, and `d` on that
one drops the line without asking.

The list lives in `recent.json` next to the config, which is why a song can be
on it without being on disk.

### Repeating a passage

The four bars that keep going wrong are the four worth playing twenty times.
Press `r` on the practice screen, and `space` marks the measure the cursor is
in: a line under the tab says which measures are marked and the head says the
passage by number. Mark as many as you like, in any order.

From there the song does not run past the passage. Play the last note of it and
the cursor is back on the first, the verdicts of the pass that ended go with it,
and in tempo mode the clock comes back a count in short of the first note, so
the passage starts with a bar to breathe in instead of under your hand. The head
counts the passes.

`r` again lets the passage go and the song runs to its end as it did. `esc`
leaves the mode and keeps the passage, since what was picked is what is being
practised. Starting the take over is `R` now, which is the one key the mode
moved.

### A tab has no rhythm in it

The text of a tab carries the notes and their order and nothing else: no
durations, no tempo, not even which of two notes side by side is the longer
one. So a song read in this way is marked as text, and tempo mode refuses it
rather than marking you wrong for playing the rhythm the song actually has.

Wait mode and the highway both work without a clock, and between them that is
most of what learning a riff is. **While the search is the only way in, every
song is text and tempo mode has nothing to run on.** The Guitar Pro reader is
still here and is not reachable from any screen; it is the only kind of tab
that carries its own tempo, and it is what that mode is waiting for.

### From a Spotify playlist

The spotify screen is one question at a time. Logged out it is a single button,
because there is nothing else to be done about it; `enter` on it opens Spotify
in your browser and keeps the session in the config folder. There is no app to
register and no client id to paste anywhere: the login is librespot, and so is
everything after it.

Not the public web api, which is the obvious way and does not work. Its quota
belongs to the client id every librespot login shares, so it answers 429 to a
single request after minutes of silence. What is read instead is what the
desktop client reads, over the same session the login opened.

Logged in, the screen is your playlists with the liked songs first, and it
reads them the first time you walk onto it, once a run. Open one and every track is looked
up on songsterr, three at a time, and the list sorts itself from easiest to
hardest with the ones you already have marked. `r` reads the library again,
which is how a playlist made on the phone turns up.

The number beside a track is the difficulty of the instrument the config screen
was told about, so changing that changes every number and the order they are
in, and the playlist on the screen is looked up again there and then.

A track carries an artist and a title and no tab at all, so `enter` on one is
the Ultimate Guitar search for it, on the music screen, which is the one way a
song gets into the library. `enter` on a track whose tab is already here opens
it for practice instead.

A track only counts as found when the artist and the title both match, ignoring
case, punctuation and the remaster note a streaming service leaves on a title.
Taking the first result instead would answer for a cover or a live version, and
report the difficulty of a transcription you were not asking about.

There is an exercise in `examples/` to try it with before reading anything in:

```bash
mkdir -p ~/fretdeck/songs && cp examples/*.json ~/fretdeck/songs/
```

## The input

The config screen lists the inputs and `r` reads the list again after something
has been plugged in.

On a machine running PipeWire the list is the sound server's own, because the
sound server holds every USB card open and the interface a guitar is plugged
into is on no list PortAudio can read. There is one ALSA device left that
reaches all of them, `pipewire`, and which source it reads is chosen per
stream. Nothing is taken exclusively, so the tuner in the browser and whatever
else is recording go on working while a song is being practised.

PortAudio also has a JACK backend, and on such a machine PipeWire answers for
JACK as well, so every input is on the list twice. The second copy is dropped
and never opened: an open there never returns, the process dies inside
PortAudio on the way out, and the client it leaves behind holds the interface
so nothing on the machine records until it is killed.

Elsewhere the list is what PortAudio sees, by index, as it always was.

That refresh is not a second query. PortAudio reads the machine once, when it
starts, and never again: an interface plugged in after the app opened is on no
list until PortAudio itself is started over, which is what `r` does. It will
not start over with a stream open, and the device that is open is missing from
the list a fresh PortAudio reads, so the input is closed first and opened again
when the list arrives. There is a moment where nothing is being heard.

If the audio side dies, and a driver taking it down is the way that happens,
the screen says so instead of waiting for an answer that cannot come. `r` on
the config screen starts it again.

A line in beats a microphone by a distance: it hears the guitar and not the room, and it does not
hear the song you are playing along to.

Playing an electric straight into the line in of the sound card works. So does
an interface, and so does a microphone in front of an amp if it is the only
thing making noise.

## Trusting the score

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

**A note has to be played, not merely waited for.** A note begins where the
level rises over whatever the room is doing, so a fan or mains hum that sits
above the silence gate and holds one pitch never becomes a note, no matter how
long it is on. Only a string being struck starts one from nothing; a note that
follows another with no gap between them, a hammer on or a slide, is heard by
the pitch changing and needs no attack of its own.

## Layout

```
fretdeck/
├── cmd/fretdeck/       the entry point
├── internal/
│   ├── song/           the song model, the tab drawing and the text tab reader
│   ├── practice/       the modes, the clock and the scoring
│   ├── songsterr/      the search, and matching a streamed track to a tab
│   ├── ultimate/       reading a tab off ultimate guitar, search and text
│   ├── bridge/         talking to the python side
│   ├── scripts/        the python side, embedded in the binary
│   ├── config/         what survives between runs, and what was played
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

## What is coming

Nothing is queued. What used to be here is done: the two steps the first run
asks, the search as the one way in, the navigation line, the mouse, the preview
of a tab before it is taken, the attack a note has to have, and `make run`.

The two that would come next are the ones the work above pointed at rather than
solved. Reading a Guitar Pro file is still in the tree and is reachable from no
screen, which is what tempo mode is waiting for, since every tab read off a
page is text and has no rhythm in it. And polyphony is still not solved and is
still not being solved: the detector names one pitch and the app is built on
that being true.
