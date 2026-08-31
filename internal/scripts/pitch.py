"""yin pitch detection over numpy, shared by the live worker and the analyzer."""

import math

import numpy as np

FRAME = 2048
HOP = 512

# the window the difference function compares against itself. tau_max is what
# is left of the frame, and it sets the lowest note the detector can name:
# 44100 / 640 is 68.9 Hz, a tone under the low E of a guitar in standard tuning
TAU_MAX = 640
WINDOW = FRAME - TAU_MAX

# 1400 Hz is above the 22nd fret of the high E string
FMAX = 1400.0

# yin absolute threshold. under it the period is accepted as it is found,
# and a frame that never gets there is reported with the confidence it has
THRESHOLD = 0.12

# below this rms the frame is silence and gets no note at all. a guitar
# decaying into nothing would otherwise keep producing octave noise
SILENCE_RMS = 0.006

NOTE_NAMES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"]


def note_name(midi):
    """turns a midi number into the name a tuner shows, C4 being middle C."""
    return "%s%d" % (NOTE_NAMES[int(midi) % 12], int(midi) // 12 - 1)


def midi_from_freq(freq):
    """fractional midi number, so the cents deviation survives the conversion."""
    return 69.0 + 12.0 * math.log2(freq / 440.0)


def freq_from_midi(midi):
    return 440.0 * (2.0 ** ((midi - 69.0) / 12.0))


def _difference(frame):
    # d(tau) = sum (x[j] - x[j+tau])^2 expanded into two power terms and a
    # correlation, because the direct sum is quadratic on the window
    x = frame.astype(np.float64)
    head = x[:WINDOW]

    size = 1 << (FRAME + WINDOW - 1).bit_length()
    correlation = np.fft.irfft(
        np.conjugate(np.fft.rfft(head, size)) * np.fft.rfft(x, size), size
    )[:TAU_MAX]

    power = np.concatenate(([0.0], np.cumsum(x * x)))
    tail = power[WINDOW : WINDOW + TAU_MAX] - power[:TAU_MAX]

    return power[WINDOW] + tail - 2.0 * correlation


def _cumulative_mean(diff):
    # the normalization that makes the threshold mean the same thing at every
    # pitch. tau zero is defined as 1 so it can never win the search
    out = np.ones_like(diff)
    running = np.cumsum(diff[1:])
    taus = np.arange(1, len(diff))
    out[1:] = diff[1:] * taus / np.maximum(running, 1e-12)
    return out


def _parabolic(values, tau):
    # the true minimum sits between samples, and without this the tuner reads
    # in steps of several cents on the high strings
    if tau <= 0 or tau >= len(values) - 1:
        return float(tau)
    a, b, c = values[tau - 1], values[tau], values[tau + 1]
    denom = a + c - 2.0 * b
    if denom == 0.0:
        return float(tau)
    return tau + 0.5 * (a - c) / denom


def detect(frame, sample_rate):
    """returns (freq, confidence, rms), with freq at zero when there is no note."""
    rms = float(np.sqrt(np.mean(frame.astype(np.float64) ** 2)))
    if rms < SILENCE_RMS:
        return 0.0, 0.0, rms

    tau_min = max(2, int(sample_rate / FMAX))
    values = _cumulative_mean(_difference(frame))

    tau = 0
    for candidate in range(tau_min, TAU_MAX):
        if values[candidate] < THRESHOLD:
            # walk down to the bottom of the dip instead of taking its first
            # sample, which is what the octave errors come from
            while candidate + 1 < TAU_MAX and values[candidate + 1] < values[candidate]:
                candidate += 1
            tau = candidate
            break

    if tau == 0:
        tau = int(np.argmin(values[tau_min:TAU_MAX])) + tau_min

    freq = sample_rate / _parabolic(values, tau)
    confidence = float(np.clip(1.0 - values[tau], 0.0, 1.0))

    if freq <= 0.0 or freq > FMAX:
        return 0.0, 0.0, rms

    return float(freq), confidence, rms


class Tracker:
    """turns a stream of frames into note starts, one event per attack.

    a frame is not a note: a single plucked string produces eighty of them and
    a few of those come out an octave off. a note is only announced once the
    same pitch has held for MIN_FRAMES in a row, which also keeps the finger
    noise between two notes from being announced as a third one.
    """

    MIN_FRAMES = 2
    MIN_CONFIDENCE = 0.55

    def __init__(self, sample_rate):
        self.sample_rate = sample_rate
        self.current = None
        self.pending = None
        self.pending_count = 0
        self.silence = 0

    def push(self, frame, time):
        """feeds one frame, returns a note dict when a new note starts."""
        freq, confidence, rms = detect(frame, self.sample_rate)

        if freq == 0.0 or confidence < self.MIN_CONFIDENCE:
            self.silence += 1
            # three silent frames is 35 ms, long enough that the same note
            # played twice in a row is heard as two notes
            if self.silence >= 3:
                self.current = None
                self.pending = None
                self.pending_count = 0
            return None

        self.silence = 0
        midi = int(round(midi_from_freq(freq)))

        if midi != self.pending:
            self.pending = midi
            self.pending_count = 1
            return None

        self.pending_count += 1
        if self.pending_count != self.MIN_FRAMES or midi == self.current:
            return None

        self.current = midi
        return {
            "t": round(time, 4),
            "midi": midi,
            "name": note_name(midi),
            "freq": round(freq, 2),
            "cents": round((midi_from_freq(freq) - midi) * 100.0, 1),
            "conf": round(confidence, 3),
            "rms": round(rms, 4),
        }
