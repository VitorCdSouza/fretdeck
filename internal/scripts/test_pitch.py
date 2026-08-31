"""tests for the detector, over signals built here instead of recordings."""

import numpy as np
import pytest

import pitch


def pluck(freq, seconds=0.2, rate=44100, noise=0.005):
    """a tone shaped like a plucked string, fundamental weaker than the second
    harmonic, which is the case the naive autocorrelation gets wrong."""
    t = np.arange(int(seconds * rate)) / rate
    signal = sum(
        amplitude * np.sin(2 * np.pi * freq * partial * t)
        for partial, amplitude in [(1, 0.4), (2, 0.7), (3, 0.35), (4, 0.15)]
    )
    signal *= np.exp(-2.0 * t)
    return (signal + noise * np.random.default_rng(0).standard_normal(len(t))).astype("float32")


# every open string of standard tuning plus two octaves over the top one
@pytest.mark.parametrize(
    "target", [82.41, 110.0, 146.83, 196.0, 246.94, 329.63, 440.0, 880.0]
)
def test_detect_names_the_pitch(target):
    frame = pluck(target)[: pitch.FRAME]
    freq, confidence, _ = pitch.detect(frame, 44100)

    assert confidence > 0.8
    assert abs(freq - target) < target * 0.005


def test_silence_is_not_a_note():
    frame = np.zeros(pitch.FRAME, dtype="float32")
    freq, confidence, rms = pitch.detect(frame, 44100)

    assert freq == 0.0
    assert confidence == 0.0
    assert rms < pitch.SILENCE_RMS


def test_note_name_puts_middle_c_in_the_fourth_octave():
    assert pitch.note_name(60) == "C4"
    assert pitch.note_name(40) == "E2"
    assert pitch.note_name(69) == "A4"


def test_midi_and_freq_are_each_other_backwards():
    for midi in range(40, 90):
        assert round(pitch.midi_from_freq(pitch.freq_from_midi(midi))) == midi


def test_tracker_reports_one_note_per_attack():
    rate = 44100
    tracker = pitch.Tracker(rate)
    audio = np.concatenate([pluck(110.0, 0.4), pluck(146.83, 0.4)])

    heard = []
    for start in range(0, len(audio) - pitch.FRAME, pitch.HOP):
        note = tracker.push(audio[start : start + pitch.FRAME], start / rate)
        if note is not None:
            heard.append(note["name"])

    assert heard == ["A2", "D3"]


def test_tracker_hears_the_same_note_twice():
    rate = 44100
    tracker = pitch.Tracker(rate)
    gap = np.zeros(int(0.1 * rate), dtype="float32")
    audio = np.concatenate([pluck(110.0, 0.3), gap, pluck(110.0, 0.3)])

    heard = 0
    for start in range(0, len(audio) - pitch.FRAME, pitch.HOP):
        if tracker.push(audio[start : start + pitch.FRAME], start / rate) is not None:
            heard += 1

    assert heard == 2
