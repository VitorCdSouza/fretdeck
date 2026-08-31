"""runs a recording of you playing against a song and reports what came out.

same detector as the live worker, so a take judged here and a take judged on
the practice screen agree. one shot: it prints a report event and exits.
"""

import argparse
import json
import sys

import numpy as np
import soundfile as sf

import pitch

# a match is worth more than a wrong note is costly, and a wrong note costs
# less than skipping over both sides, so a fret played wrong lines up with the
# note it was meant to be instead of turning into a miss plus an extra
MATCH = 2.0
MISMATCH = -1.0
GAP = -2.0


def emit(event, message="", data=None):
    line = {"event": event, "message": message}
    if data is not None:
        line["data"] = data
    sys.stdout.write(json.dumps(line) + "\n")
    sys.stdout.flush()


def expected_events(song):
    """collapses the notes into one event per attack, so a chord is one event.

    the detector is monophonic and a strummed chord gives back one pitch out of
    it, whichever rings loudest. counting the chord as six separate demands
    would score every chord in the song as one hit and five misses.
    """
    events = []
    for note in song["notes"]:
        if events and abs(note["time"] - events[-1]["time"]) < 0.02:
            events[-1]["midis"].append(note["midi"])
            events[-1]["frets"].append([note["string"], note["fret"]])
            continue
        events.append(
            {
                "time": note["time"],
                "measure": note["measure"],
                "midis": [note["midi"]],
                "frets": [[note["string"], note["fret"]]],
            }
        )
    return events


def prune(played):
    """drops a reading that was replaced almost at once.

    the first frames of a plucked low string often read an octave down, before
    the fundamental settles, and that reading would be reported as a note the
    player added. nothing on a guitar is played and abandoned in 50 ms
    """
    kept = []
    for index, note in enumerate(played):
        if index + 1 < len(played) and played[index + 1]["t"] - note["t"] < 0.05:
            continue
        kept.append(note)
    return kept


def transcribe(path, progress=True):
    """the notes heard in the file, in order."""
    data, rate = sf.read(path, dtype="float32", always_2d=True)
    mono = data.mean(axis=1)

    tracker = pitch.Tracker(rate)
    played = []
    total = max(1, (len(mono) - pitch.FRAME) // pitch.HOP)
    step = max(1, total // 20)

    for count, start in enumerate(range(0, len(mono) - pitch.FRAME, pitch.HOP)):
        frame = mono[start : start + pitch.FRAME]
        note = tracker.push(frame, start / rate)
        if note is not None:
            played.append(note)
        if progress and count % step == 0:
            emit("progress", data={"done": count, "total": total})

    return prune(played), len(mono) / rate


def matches(midi, expected):
    if midi in expected:
        return True
    # the low strings often read one octave up, because the fundamental of a
    # wound string is quieter than its second harmonic. that is the detector
    # being a detector, not the wrong fret
    return any(abs(midi - value) == 12 for value in expected)


def align(events, played):
    """needleman wunsch over the two note sequences.

    a plain index by index walk breaks on the first extra or skipped note and
    reports every note after it as wrong, which says nothing useful about a
    take where one note was missed in the second measure.
    """
    rows, columns = len(events), len(played)
    scores = np.zeros(columns + 1)
    scores[:] = np.arange(columns + 1) * GAP
    # 0 diagonal, 1 up (an expected note nobody played), 2 left (an extra note)
    pointers = np.zeros((rows + 1, columns + 1), dtype=np.uint8)
    pointers[0, 1:] = 2
    pointers[1:, 0] = 1

    for row in range(1, rows + 1):
        expected = events[row - 1]["midis"]
        hit = np.array(
            [MATCH if matches(note["midi"], expected) else MISMATCH for note in played]
        )

        previous = scores
        scores = np.empty(columns + 1)
        scores[0] = row * GAP

        # the left move reads the cell being written, so this row cannot be
        # vectorized past the diagonal and the up move
        best_diagonal = previous[:-1] + hit
        best_up = previous[1:] + GAP

        for column in range(1, columns + 1):
            diagonal = best_diagonal[column - 1]
            up = best_up[column - 1]
            left = scores[column - 1] + GAP

            if diagonal >= up and diagonal >= left:
                scores[column] = diagonal
                pointers[row, column] = 0
            elif up >= left:
                scores[column] = up
                pointers[row, column] = 1
            else:
                scores[column] = left
                pointers[row, column] = 2

    steps = []
    row, column = rows, columns
    while row > 0 or column > 0:
        move = pointers[row, column]
        if row > 0 and column > 0 and move == 0:
            steps.append(("pair", row - 1, column - 1))
            row -= 1
            column -= 1
        elif row > 0 and move == 1:
            steps.append(("missed", row - 1, None))
            row -= 1
        else:
            steps.append(("extra", None, column - 1))
            column -= 1

    steps.reverse()
    return steps


def report(song, events, played, duration, steps):
    results = []
    hits = 0
    offsets = []

    for kind, index, position in steps:
        if kind == "extra":
            results.append(
                {
                    "kind": "extra",
                    "played": played[position]["name"],
                    "at": played[position]["t"],
                }
            )
            continue

        event = events[index]
        entry = {
            "measure": event["measure"],
            "time": event["time"],
            "frets": event["frets"],
            "expected": [pitch.note_name(midi) for midi in event["midis"]],
        }

        if kind == "missed":
            entry["kind"] = "missed"
        else:
            note = played[position]
            entry["at"] = note["t"]
            entry["played"] = note["name"]
            if matches(note["midi"], event["midis"]):
                entry["kind"] = "hit"
                hits += 1
                offsets.append(note["t"] - event["time"])
            else:
                entry["kind"] = "wrong"

        results.append(entry)

    total = len(events)
    span = events[-1]["time"] - events[0]["time"] if total > 1 else 0.0

    summary = {
        "notes": total,
        "hits": hits,
        "accuracy": round(hits / total, 4) if total else 0.0,
        "extras": sum(1 for step in steps if step[0] == "extra"),
        "missed": sum(1 for step in steps if step[0] == "missed"),
        "duration": round(duration, 2),
    }

    # how fast the take actually was, which is the one number that explains a
    # run where every note is right and it still sounded wrong
    if len(offsets) > 1 and span > 0:
        first, last = offsets[0], offsets[-1]
        summary["tempo"] = round(song["tempo"] * span / (span + last - first), 1)

    by_measure = {}
    for entry in results:
        if entry["kind"] == "extra":
            continue
        bucket = by_measure.setdefault(entry["measure"], {"notes": 0, "hits": 0})
        bucket["notes"] += 1
        bucket["hits"] += entry["kind"] == "hit"

    return {
        "song": song["title"],
        "summary": summary,
        "measures": [
            {"index": index, "notes": value["notes"], "hits": value["hits"]}
            for index, value in sorted(by_measure.items())
        ],
        "notes": results,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--song", required=True)
    parser.add_argument("--audio", required=True)
    args = parser.parse_args()

    with open(args.song, encoding="utf-8") as handle:
        song = json.load(handle)

    events = expected_events(song)
    if not events:
        emit("analyze_error", "the song has no notes to compare against")
        return 1

    try:
        played, duration = transcribe(args.audio)
    except Exception as error:
        emit("analyze_error", str(error))
        return 1

    if not played:
        emit("analyze_error", "no note was heard in the file")
        return 1

    emit("report", data=report(song, events, played, duration, align(events, played)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
