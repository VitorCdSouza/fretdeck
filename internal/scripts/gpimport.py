"""reads a guitar pro file and writes the song json the tui plays from.

one shot: the tui starts it, reads the events, and the process ends. a guitar
pro file is the only tab source with real timing in it, which is what the tempo
mode needs and what an ascii tab off the internet can never provide.
"""

import argparse
import json
import sys

import guitarpro

# guitar pro counts a quarter note as 960 ticks and starts the first measure
# at one quarter, not at zero
QUARTER = 960.0


def emit(event, message="", data=None):
    line = {"event": event, "message": message}
    if data is not None:
        line["data"] = data
    sys.stdout.write(json.dumps(line) + "\n")
    sys.stdout.flush()


def playable(track):
    # a percussion track has no strings to put on a tab, and a keyboard track
    # has none either
    return bool(track.strings) and not track.isPercussionTrack


def describe(song):
    return [
        {
            "index": index,
            "name": track.name,
            "strings": len(track.strings),
            "playable": playable(track),
            "measures": len(track.measures),
        }
        for index, track in enumerate(song.tracks)
    ]


def tempo_map(song):
    """every tempo in the file as (tick, bpm), in order and without repeats.

    a tempo change does not live on the measure: it rides on the beat where it
    happens, inside a mix table change, and it can land in the middle of one.
    collecting the changes first turns any tick into seconds with one pass.
    """
    changes = {0.0: float(song.tempo)}

    for track in song.tracks:
        for measure in track.measures:
            for voice in measure.voices:
                for beat in voice.beats:
                    change = beat.effect.mixTableChange
                    if change is not None and change.tempo is not None:
                        changes[float(beat.start - QUARTER)] = float(change.tempo.value)

    return sorted(changes.items())


def seconds_at(changes, tick):
    """integrates the tempo map up to tick, since bpm is not constant."""
    seconds = 0.0
    for index, (start, tempo) in enumerate(changes):
        if start >= tick:
            break
        end = changes[index + 1][0] if index + 1 < len(changes) else tick
        seconds += (min(end, tick) - start) * 60.0 / (tempo * QUARTER)
    return seconds


def tempo_at(changes, tick):
    tempo = changes[0][1]
    for start, value in changes:
        if start > tick:
            break
        tempo = value
    return tempo


def tapped(beat):
    """whether the beat is tapped, which guitar pro keeps on the beat and not
    on the note, in the same field as the slap and the pop."""
    slap = getattr(beat.effect, "slapEffect", None)
    return slap is not None and getattr(slap, "name", "") == "tapping"


def technique(beat, note, ringing):
    """the one technique the note is worth, hardest first.

    a note carries several of them at once and the app puts each on one rung of
    a ladder, so the hardest is the one that says what the note is worth. the
    ones that are only a way of ringing a note that was already struck come
    last, since a bend over a hammer on is still a bend to play.
    """
    effect = note.effect

    if effect.bend is not None:
        return "bend"
    if effect.harmonic is not None:
        return "harmonic"
    if tapped(beat):
        return "tap"
    if effect.vibrato:
        return "vibrato"
    if effect.palmMute:
        return "palm"
    if effect.slides:
        return "slide"

    if effect.hammer:
        # guitar pro writes one flag for both, and which of the two it is is
        # the direction: a note over the one still ringing is hammered on
        held = ringing.get(note.string)
        if held is not None and note.value < held["fret"]:
            return "pull"
        return "hammer"

    return ""


def convert(song, index):
    track = song.tracks[index]
    if not playable(track):
        raise ValueError("track %d has no strings to draw a tab from" % index)

    changes = tempo_map(song)
    open_midi = {string.number: string.value for string in track.strings}

    notes = []
    measures = []
    ringing = {}

    for measure in track.measures:
        header = measure.header
        origin = measure.start - QUARTER

        measures.append(
            {
                "index": header.number,
                "beat": round(origin / QUARTER, 4),
                "time": round(seconds_at(changes, origin), 4),
                "tempo": tempo_at(changes, origin),
                "signature": [
                    header.timeSignature.numerator,
                    header.timeSignature.denominator.value,
                ],
            }
        )

        for voice in measure.voices:
            for beat in voice.beats:
                if beat.status != guitarpro.BeatStatus.normal:
                    continue

                start = beat.start - QUARTER
                start_time = seconds_at(changes, start)
                end_time = seconds_at(changes, start + beat.duration.time)

                for note in beat.notes:
                    # a dead note is a muted click with no pitch to hear, so
                    # asking the player to match it would never end
                    if note.type == guitarpro.NoteType.dead:
                        continue

                    if note.type == guitarpro.NoteType.tie:
                        # a tie is the same note still ringing, so it stretches
                        # the one before it on that string instead of asking to
                        # be played again
                        held = ringing.get(note.string)
                        if held is not None:
                            held["dur"] = round(end_time - held["time"], 4)
                        continue

                    how = technique(beat, note, ringing)
                    entry = {
                        "measure": header.number,
                        "beat": round(start / QUARTER, 4),
                        "time": round(start_time, 4),
                        "dur": round(end_time - start_time, 4),
                        "string": note.string,
                        "fret": note.value,
                        "midi": open_midi[note.string] + note.value,
                    }
                    # a note picked plain carries no key at all, so a file
                    # written before this reads back the same either way
                    if how:
                        entry["tech"] = how

                    notes.append(entry)
                    ringing[note.string] = entry

    # the low strings come first inside a chord, which is the order a downward
    # strum hits them and the order the tab is read in
    notes.sort(key=lambda note: (note["time"], -note["string"]))

    return {
        "title": song.title or "untitled",
        "artist": song.artist or "",
        "track": track.name,
        "tempo": float(song.tempo),
        "tuning": [
            {"string": string.number, "midi": string.value}
            for string in track.strings
        ],
        "measures": measures,
        "notes": notes,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("file")
    parser.add_argument("--track", type=int)
    parser.add_argument("--out")
    args = parser.parse_args()

    try:
        song = guitarpro.parse(args.file)
    except Exception as error:
        emit("import_error", str(error))
        return 1

    if args.track is None:
        emit("tracks", data={"title": song.title, "tracks": describe(song)})
        return 0

    try:
        converted = convert(song, args.track)
    except Exception as error:
        emit("import_error", str(error))
        return 1

    with open(args.out, "w", encoding="utf-8") as handle:
        json.dump(converted, handle, ensure_ascii=False, indent=1)

    emit(
        "imported",
        data={
            "path": args.out,
            "title": converted["title"],
            "notes": len(converted["notes"]),
            "measures": len(converted["measures"]),
        },
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
