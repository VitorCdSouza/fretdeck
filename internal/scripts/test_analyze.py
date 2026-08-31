"""tests for the marking of a recording, over takes rendered here."""

import numpy as np
import pytest
import soundfile as sf

import analyze
import pitch


def song():
    notes = []
    for index, midi in enumerate([43, 45, 48, 50]):
        notes.append(
            {
                "measure": index // 2 + 1,
                "beat": float(index),
                "time": index * 0.5,
                "dur": 0.5,
                "string": 6,
                "fret": midi - 40,
                "midi": midi,
            }
        )
    return {"title": "fixture", "tempo": 120.0, "notes": notes}


def take(tmp_path, midis, rate=44100, spacing=0.5):
    """renders somebody playing those notes, one every spacing seconds."""
    total = int((len(midis) * spacing + 0.5) * rate)
    buffer = np.zeros(total, dtype="float32")

    for index, midi in enumerate(midis):
        freq = pitch.freq_from_midi(midi)
        t = np.arange(int(0.4 * rate)) / rate
        tone = sum(
            amplitude * np.sin(2 * np.pi * freq * partial * t)
            for partial, amplitude in [(1, 0.4), (2, 0.7), (3, 0.3)]
        ) * np.exp(-3.0 * t)
        at = int(index * spacing * rate)
        end = min(total, at + len(tone))
        buffer[at:end] += tone[: end - at].astype("float32")

    path = tmp_path / "take.wav"
    sf.write(str(path), buffer, rate)
    return str(path)


def test_a_chord_is_one_event():
    written = song()
    written["notes"][1]["time"] = written["notes"][0]["time"]

    events = analyze.expected_events(written)

    assert len(events) == 3
    assert events[0]["midis"] == [43, 45]


def test_matches_accepts_the_octave_the_detector_reports():
    assert analyze.matches(43, [43])
    assert analyze.matches(55, [43])
    assert not analyze.matches(44, [43])


def test_prune_drops_a_reading_that_was_replaced_at_once():
    played = [
        {"t": 0.0, "midi": 31},
        {"t": 0.02, "midi": 43},
        {"t": 0.5, "midi": 45},
    ]

    assert [note["midi"] for note in analyze.prune(played)] == [43, 45]


def test_a_clean_take_is_marked_clean(tmp_path):
    written = song()
    events = analyze.expected_events(written)
    played, duration = analyze.transcribe(take(tmp_path, [43, 45, 48, 50]), progress=False)

    report = analyze.report(written, events, played, duration, analyze.align(events, played))

    assert report["summary"]["hits"] == 4
    assert report["summary"]["accuracy"] == 1.0
    assert report["summary"]["extras"] == 0


def test_a_wrong_note_is_named(tmp_path):
    written = song()
    events = analyze.expected_events(written)
    # the third note played a semitone off. the pitch is one the song never
    # asks for, or the alignment would be right to read it as another note
    played, duration = analyze.transcribe(take(tmp_path, [43, 45, 44, 50]), progress=False)

    report = analyze.report(written, events, played, duration, analyze.align(events, played))
    kinds = [note["kind"] for note in report["notes"]]

    assert kinds == ["hit", "hit", "wrong", "hit"]

    wrong = report["notes"][2]
    assert wrong["expected"] == ["C3"]
    assert wrong["played"] == "G#2"


def test_the_alignment_survives_a_skipped_note(tmp_path):
    written = song()
    events = analyze.expected_events(written)
    # the second note missing. an index by index walk would call the two after
    # it wrong as well
    played, duration = analyze.transcribe(take(tmp_path, [43, 48, 50]), progress=False)

    report = analyze.report(written, events, played, duration, analyze.align(events, played))

    assert report["summary"]["hits"] == 3
    assert report["summary"]["missed"] == 1


def test_the_report_says_how_fast_the_take_was(tmp_path):
    written = song()
    events = analyze.expected_events(written)
    # every note a fifth late on the one before it, so the take is slower
    played, duration = analyze.transcribe(
        take(tmp_path, [43, 45, 48, 50], spacing=0.6), progress=False
    )

    report = analyze.report(written, events, played, duration, analyze.align(events, played))

    assert report["summary"]["tempo"] == pytest.approx(100, abs=6)


def test_measures_are_counted_apart(tmp_path):
    written = song()
    events = analyze.expected_events(written)
    played, duration = analyze.transcribe(take(tmp_path, [43, 45, 48, 50]), progress=False)

    report = analyze.report(written, events, played, duration, analyze.align(events, played))

    assert [measure["index"] for measure in report["measures"]] == [1, 2]
    assert all(measure["notes"] == 2 for measure in report["measures"])
