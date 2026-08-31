"""tests for the guitar pro importer, over files written here."""

import guitarpro
import pytest

import gpimport

QUARTER = gpimport.QUARTER


def write(tmp_path, riff, tempo=120, name="Guitar 1"):
    """builds a one measure track out of (string, fret) pairs of eighth notes."""
    song = guitarpro.Song()
    song.title = "fixture"
    song.artist = "nobody"
    song.tempo = tempo

    track = song.tracks[0]
    track.name = name

    measure = track.measures[0]
    voice = measure.voices[0]
    voice.beats = []
    start = measure.start

    for string, fret, kind in riff:
        beat = guitarpro.Beat(voice)
        beat.status = guitarpro.BeatStatus.normal
        beat.duration = guitarpro.Duration(value=8)
        beat.start = start
        note = guitarpro.Note(beat)
        note.string = string
        note.value = fret
        note.type = kind
        beat.notes = [note]
        voice.beats.append(beat)
        start += beat.duration.time

    path = tmp_path / "fixture.gp5"
    guitarpro.write(song, str(path))
    return str(path)


def test_seconds_at_integrates_over_a_tempo_change():
    # 120 bpm for one quarter, then 60 bpm. the first quarter takes half a
    # second and the second one takes a whole second
    changes = [(0.0, 120.0), (QUARTER, 60.0)]

    assert gpimport.seconds_at(changes, QUARTER) == pytest.approx(0.5)
    assert gpimport.seconds_at(changes, 2 * QUARTER) == pytest.approx(1.5)


def test_tempo_at_holds_until_the_next_change():
    changes = [(0.0, 120.0), (2 * QUARTER, 60.0)]

    assert gpimport.tempo_at(changes, QUARTER) == 120.0
    assert gpimport.tempo_at(changes, 2 * QUARTER) == 60.0
    assert gpimport.tempo_at(changes, 9 * QUARTER) == 60.0


def test_convert_places_the_notes_in_seconds(tmp_path):
    normal = guitarpro.NoteType.normal
    path = write(tmp_path, [(6, 3, normal), (6, 5, normal), (5, 0, normal)], tempo=120)

    converted = gpimport.convert(guitarpro.parse(path), 0)
    times = [note["time"] for note in converted["notes"]]

    # an eighth note at 120 bpm lasts a quarter of a second
    assert times == pytest.approx([0.0, 0.25, 0.5])
    assert [note["midi"] for note in converted["notes"]] == [43, 45, 45]
    assert converted["tuning"][0] == {"string": 1, "midi": 64}


def test_a_dead_note_is_not_asked_for(tmp_path):
    normal, dead = guitarpro.NoteType.normal, guitarpro.NoteType.dead
    path = write(tmp_path, [(6, 3, normal), (6, 5, dead), (6, 7, normal)], tempo=120)

    converted = gpimport.convert(guitarpro.parse(path), 0)

    assert [note["fret"] for note in converted["notes"]] == [3, 7]


def test_a_tie_stretches_the_note_before_it(tmp_path):
    normal, tie = guitarpro.NoteType.normal, guitarpro.NoteType.tie
    path = write(tmp_path, [(6, 3, normal), (6, 3, tie)], tempo=120)

    converted = gpimport.convert(guitarpro.parse(path), 0)

    assert len(converted["notes"]) == 1
    assert converted["notes"][0]["dur"] == pytest.approx(0.5)


def test_describe_marks_a_track_with_no_strings(tmp_path):
    path = write(tmp_path, [(6, 3, guitarpro.NoteType.normal)])
    song = guitarpro.parse(path)
    song.tracks[0].isPercussionTrack = True

    assert gpimport.describe(song)[0]["playable"] is False
