"""tests for the metronome, over buffers built here and never a sound card."""

import numpy as np

import pitch
import worker


RATE = 44100


def test_a_beat_is_as_long_as_the_tempo_says():
    beat = worker.one_beat(RATE, 120.0, 1)

    assert len(beat) == RATE // 2


def test_the_first_beat_of_the_bar_is_the_other_tone():
    accent = worker.one_beat(RATE, 120.0, 0)
    plain = worker.one_beat(RATE, 120.0, 1)

    assert not np.array_equal(accent[:1000], plain[:1000])


def test_the_click_is_a_burst_and_then_silence():
    beat = worker.one_beat(RATE, 60.0, 1)
    burst = int(RATE * worker.CLICK_SECONDS)

    assert np.max(np.abs(beat[:burst])) > 0.1
    assert np.max(np.abs(beat[burst:])) == 0.0


def test_the_click_is_never_a_note_anybody_plays():
    """the whole of why the tone was picked where it was: the pitches the
    detector can mistake it for are pitches no string is ever tuned to."""
    for one in pitch.CLICK_PITCHES:
        assert abs(one - round(one)) - pitch.CLICK_TOLERANCE > 0.1


def test_the_tracker_stays_deaf_to_the_click_and_not_to_a_string():
    tracker = pitch.Tracker(RATE)
    tracker.deaf = pitch.CLICK_PITCHES

    for one in pitch.CLICK_PITCHES:
        assert tracker.deafened(pitch.freq_from_midi(one))

    # every note a guitar in standard tuning can play, open string to the
    # twenty fourth fret of the top one
    for midi in range(40, 89):
        assert not tracker.deafened(pitch.freq_from_midi(midi))


def test_nothing_is_deaf_while_the_click_is_off():
    tracker = pitch.Tracker(RATE)

    assert not tracker.deafened(pitch.freq_from_midi(pitch.CLICK_PITCHES[0]))


def test_the_metronome_is_off_until_it_is_asked_for():
    metronome = worker.Metronome(RATE)
    assert not metronome.sounding()

    metronome.set(90, 3)
    assert metronome.sounding()
    assert metronome.beats == 3

    metronome.set(0, 4)
    assert not metronome.sounding()


class FakeStream:
    """an output that writes nowhere, so the loop can be run on a machine with
    no sound card and on a runner with no sound at all."""

    def __init__(self, written, **kwargs):
        self.written = written

    def start(self):
        pass

    def write(self, block):
        self.written.append(len(block))

    def abort(self, ignore_errors=True):
        pass

    def close(self, ignore_errors=True):
        pass


def test_the_loop_writes_a_beat_at_a_time_until_it_is_stopped(monkeypatch):
    written = []
    monkeypatch.setattr(
        worker.sd, "OutputStream", lambda **kwargs: FakeStream(written, **kwargs)
    )

    metronome = worker.Metronome(RATE)
    metronome.set(120, 4)

    def stop_after_a_bar(block):
        written.append(len(block))
        if len(written) == 4:
            metronome.stop_flag.set()

    monkeypatch.setattr(FakeStream, "write", lambda self, block: stop_after_a_bar(block))
    metronome.run()

    assert written == [RATE // 2] * 4


def test_an_output_that_cannot_be_opened_says_so_and_stops_asking(monkeypatch):
    said = []
    monkeypatch.setattr(worker, "emit", lambda event, message="", data=None: said.append(event))

    metronome = worker.Metronome(RATE)

    def refuse(**kwargs):
        metronome.stop_flag.set()
        raise OSError("no output")

    monkeypatch.setattr(worker.sd, "OutputStream", refuse)

    metronome.set(120, 4)
    metronome.run()

    assert said == ["click_error"]
    assert not metronome.sounding()
