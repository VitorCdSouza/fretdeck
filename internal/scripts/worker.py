"""the live audio side: opens an input device and reports what is being played.

stays alive for the whole session and takes one json command per line on
stdin, because opening a device costs a fraction of a second and doing it on
every note would be heard as a gap.
"""

import json
import os
import queue
import subprocess
import sys
import threading
import time

import numpy as np
import sounddevice as sd

import pitch

# the tuner needs a reading even while the same note is held, and twenty a
# second is smooth to the eye without flooding the pipe with json
LEVEL_INTERVAL = 0.05

# the alsa device every source on the machine can be reached through
PIPEWIRE = "pipewire"

# how far behind the clock the audio may fall before the input is taken as
# dead. it counts what the device delivered and not what was read out of the
# queue, or a slow reader would look like a broken interface
STALL = 2.0

# how often the source is looked for while a stream is reading it
WATCH = 2.0

# what a failed open waits before opening again, and the ceiling of that
RETRY = 0.5
RETRY_MAX = 3.0

# how many stalls in a row are a device with something wrong with it rather
# than one hiccup, and how long a stretch of audio ends that streak
KEEPS_STALLING = 4
STEADY = 30.0

# portaudio lists the whole machine again through it and opens none of it
JACK = "JACK Audio Connection Kit"

# how long one click lasts and how loud it is. twenty milliseconds is heard as
# a click and is shorter than one frame of the detector, so it can only ever
# spoil the one frame it lands in
CLICK_SECONDS = 0.02
CLICK_LEVEL = 0.25

# the edges of the burst are faded over this, since a tone that starts on a
# step is a click plus a thump and a thump has no pitch to stay deaf to
CLICK_FADE = 0.002


def emit(event, message="", data=None):
    line = {"event": event, "message": message}
    if data is not None:
        line["data"] = data
    try:
        sys.stdout.write(json.dumps(line) + "\n")
        sys.stdout.flush()
    except (BrokenPipeError, ValueError):
        # the app is gone, and there is nothing left here worth doing
        os._exit(0)


def rescan():
    """starts portaudio over, which is the only way the device list changes.

    it reads the machine when it is initialized and never again, so an
    interface plugged in after this worker started is on no list it can answer.
    it will not start over with a stream open, and the device that is open is
    missing from the list a fresh portaudio reads, which is why the caller
    stops the stream first and the app opens it again afterwards.
    """
    try:
        sd._terminate()
        sd._initialize()
    except Exception as error:
        # an old sounddevice without them still answers the list it has
        emit("log", "could not start portaudio over: %s" % error)


def alsa_device(name):
    """the index of an alsa device by the name portaudio gives it.

    sounddevice takes a name and matches it against every backend, so asking
    for one by string can hand back the jack copy of it instead. the host api
    is half of what identifies a device and is checked here.
    """
    hosts = sd.query_hostapis()

    for index, device in enumerate(sd.query_devices()):
        if device["max_input_channels"] < 1:
            continue
        if hosts[device["hostapi"]]["name"] == "ALSA" and device["name"] == name:
            return index, device

    return None, None


def pipewire_dump():
    try:
        done = subprocess.run(
            ["pw-dump"], capture_output=True, text=True, timeout=5.0
        )
    except (OSError, subprocess.SubprocessError):
        return []

    if done.returncode != 0:
        return []

    try:
        return json.loads(done.stdout)
    except ValueError:
        return []


def card_of(props):
    """what a source is plugged into, which a profile change does not rename.

    the node name carries the profile in it, so the same pedal is
    `...analog-stereo` under duplex and `...pro-input-0` under pro audio. the
    card is the same string either way and is what a saved input is found by
    when the name it was saved under is gone.
    """
    return (
        props.get("api.alsa.card.longname")
        or props.get("alsa.long_card_name")
        or props.get("device.name")
        or props.get("node.name")
        or ""
    )


def stem(name):
    """a node name without the profile on the end of it.

    it is what an input saved before the card was written down is found by,
    and a name of one part has none, since `bluez_input` alone is every
    bluetooth input on the machine and not one of them.
    """
    if name.count(".") < 2:
        return ""
    return name.rsplit(".", 1)[0]


def find_source(sources, name, card):
    """the input that was chosen, by its name, its card and then its stem.

    a card switched between profiles answers under another node name, and the
    saved one is on no list until the profile is put back by hand, which is
    what made the same pedal work one run and not the next.
    """
    for source in sources:
        if name and source["id"] == name:
            return source

    for source in sources:
        if card and source.get("card") == card:
            return source

    root = stem(name)
    for source in sources:
        if root and stem(source["id"]) == root:
            return source

    return None


def pipewire_sources(rate):
    """the inputs pipewire knows about, named the way it names them.

    the sound server holds every usb card open, so the card itself is missing
    from the list portaudio reads and the interface somebody plugs a guitar
    into cannot be picked there at all. the one alsa device that is left is
    `pipewire`, and which source it reads is chosen per stream, which is what
    this list is for. a node name survives a replug and an index does not.
    """
    default = ""
    sources = []

    for entry in pipewire_dump():
        if entry.get("type", "").endswith("Metadata"):
            for item in entry.get("metadata", []):
                if item.get("key") == "default.audio.source":
                    value = item.get("value")
                    if isinstance(value, dict):
                        default = value.get("name", "")
            continue

        if not entry.get("type", "").endswith("Node"):
            continue

        props = entry.get("info", {}).get("props", {})
        if props.get("media.class") != "Audio/Source":
            continue

        name = props.get("node.name")
        if not name:
            continue

        sources.append(
            {
                "id": name,
                "card": card_of(props),
                "name": props.get("node.description") or name,
                "host": "PipeWire",
                "channels": int(props.get("audio.channels") or 1),
                "rate": rate,
            }
        )

    for source in sources:
        source["default"] = source["id"] == default

    return sources


def portaudio_devices():
    """what portaudio answers, minus the backend that cannot be opened here.

    a jack device is every source on the machine over again, and opening one
    of them never returns: the open sits in Pa_OpenStream and the process
    dies asserting inside Pa_Terminate, leaving a client behind that holds the
    interface so nothing else on the machine records either.
    """
    hosts = sd.query_hostapis()
    devices = []
    default = sd.default.device[0]

    for index, device in enumerate(sd.query_devices()):
        if device["max_input_channels"] < 1:
            continue

        host = hosts[device["hostapi"]]["name"]
        if host == JACK:
            continue

        devices.append(
            {
                "id": "",
                "index": index,
                "name": device["name"],
                "host": host,
                "channels": device["max_input_channels"],
                "rate": int(device["default_samplerate"]),
                "default": index == default,
            }
        )

    return devices


def list_devices():
    rescan()

    index, device = alsa_device(PIPEWIRE)
    if index is not None:
        sources = pipewire_sources(int(device["default_samplerate"]))
        if sources:
            for source in sources:
                source["index"] = index
            return sources

    return portaudio_devices()


def aim(source):
    """points the alsa pipewire plugin at one source, by name.

    the plugin reads it when a stream is opened and not before, so an input
    picked on the config screen takes on the next open. nothing set is the
    source the system is pointed at, which is what a machine without pipewire
    gets anyway.
    """
    if source:
        os.environ["PIPEWIRE_PROPS"] = '{ target.object = "%s" }' % source
    else:
        os.environ.pop("PIPEWIRE_PROPS", None)


def revive(source):
    """suspends the source and lets it back, which restarts the pcm under it.

    a usb codec whose capture stops feeding leaves the node running and every
    client on it reading nothing, and a client opening its own stream again
    does not touch that: the sound server is holding the card open the whole
    time. this is the profile switch people do by hand, only narrower, and it
    is aimed at the one input that was chosen.
    """
    if not source:
        return False

    for state in ("1", "0"):
        try:
            done = subprocess.run(
                ["pactl", "suspend-source", source, state],
                capture_output=True, timeout=5.0
            )
        except (OSError, subprocess.SubprocessError):
            return False
        if done.returncode != 0:
            return False
        time.sleep(0.5)

    return True


def shut(stream):
    """drops a stream instead of draining it.

    closing the polite way one whose device stopped feeding takes twenty five
    seconds, and the app is deaf for every one of them.
    """
    try:
        stream.abort(ignore_errors=True)
    finally:
        stream.close(ignore_errors=True)


def click_tone(rate, midi):
    """one click: a short burst of a tone that is not a note.

    the pitch is deliberately half a semitone off the grid a fretboard is
    written on, which is what lets the tracker stay deaf to it without ever
    being deaf to a string.
    """
    count = max(1, int(rate * CLICK_SECONDS))
    steps = np.arange(count)

    wave = np.sin(2 * np.pi * pitch.freq_from_midi(midi) * steps / float(rate))
    edge = max(1.0, CLICK_FADE * rate)
    fade = np.minimum(1.0, np.minimum(steps, steps[::-1]) / edge)

    return (wave * fade * CLICK_LEVEL).astype("float32")


def one_beat(rate, bpm, index):
    """the audio of one beat: the click and the silence after it.

    a beat at a time and not a bar, so a click that is turned off is quiet
    within a beat rather than at the end of the bar it was in.
    """
    length = max(1, int(round(rate * 60.0 / bpm)))
    out = np.zeros(length, dtype="float32")

    tone = click_tone(rate, pitch.ACCENT_MIDI if index == 0 else pitch.CLICK_MIDI)
    out[: len(tone)] = tone[:length]

    return out


class Metronome:
    """the click, on an output of its own.

    it keeps its own time. a beat handed over the pipe one at a time would
    carry every hiccup of the pipe with it, so what the go side sends is the
    beat and the bar and the writing here is what paces it: a blocking stream
    is consumed at exactly the rate the card runs at.
    """

    def __init__(self, rate=44100):
        self.rate = rate
        self.bpm = 0.0
        self.beats = 4
        self.changed = threading.Event()
        self.stop_flag = threading.Event()

    def set(self, bpm, beats):
        """asks for a beat, and a bpm of zero is what turns it off."""
        self.bpm = max(0.0, float(bpm or 0.0))
        self.beats = max(1, int(beats or 4))
        self.changed.set()

    def sounding(self):
        return self.bpm > 0.0

    def run(self):
        while not self.stop_flag.is_set():
            self.changed.clear()
            bpm, beats = self.bpm, self.beats

            if bpm <= 0.0:
                self.changed.wait(0.2)
                continue

            try:
                stream = sd.OutputStream(
                    samplerate=self.rate, channels=1, dtype="float32"
                )
                stream.start()
            except Exception as error:
                # nothing to play it on is the click not working and not the
                # listening, so it says so and stops asking
                emit("click_error", str(error))
                self.bpm = 0.0
                continue

            try:
                index = 0
                while not self.stop_flag.is_set() and not self.changed.is_set():
                    stream.write(one_beat(self.rate, bpm, index))
                    index = (index + 1) % beats
            except Exception as error:
                emit("click_error", str(error))
                self.bpm = 0.0
            finally:
                shut(stream)


class Listener:
    """one open input stream, feeding the tracker frame by frame."""

    def __init__(self, device, rate, source="", card="", metronome=None):
        self.device = device
        self.rate = rate
        self.source = source
        self.card = card
        self.metronome = metronome
        self.blocks = queue.Queue()
        self.stream = None
        self.stop_flag = threading.Event()
        self.done = threading.Event()

        # the source going away under an open stream, which raises nothing
        self.lost = threading.Event()
        self.frames = 0

    def _callback(self, indata, frames, time_info, status):
        if status:
            # an overflow means the reader is late, and the audio it dropped is
            # gone. saying so beats reporting notes that were never played
            emit("audio_warning", str(status))
        self.frames += frames
        # only the first channel: a stereo interface would otherwise average
        # the guitar with whatever is on the other input
        self.blocks.put(indata[:, 0].copy())

    def run(self):
        """opens the input and keeps it open for as long as it is wanted.

        an input that cannot be opened yet, one unplugged mid session and one
        renamed by a profile change all end here, and all three come back on
        their own, so none of them is the end of listening.
        """
        waiting = ""
        delay = RETRY
        stalls = 0

        while not self.stop_flag.is_set():
            try:
                self.stream = self._open()
            except Exception as error:
                if str(error) != waiting:
                    waiting = str(error)
                    emit("listen_waiting", waiting)
                self.stop_flag.wait(delay)
                delay = min(delay * 2, RETRY_MAX)
                continue

            waiting, delay = "", RETRY
            began = time.monotonic()
            try:
                why = self._loop()
            except Exception as error:
                # the reading itself broke, which is worth saying, and the
                # stream is opened again all the same
                emit("listen_error", str(error))
                why = "stall"
            finally:
                # portaudio cannot be restarted with a stream open, so whoever
                # stops this one has to know it is really shut
                self.stream = None

            if why != "stall":
                stalls = 0
                continue

            if time.monotonic() - began > STEADY:
                stalls = 0
            stalls += 1
            self._jammed(stalls)

        self.done.set()
        emit("stopped")

    def _jammed(self, stalls):
        """what to do about an input that stopped feeding the stream.

        opening the stream again is not enough on its own: the sound server
        holds the card open and goes on handing out an input that has stopped,
        so the source is put down and taken back up first.
        """
        # one hiccup is only opened again: the sound server is poked when the
        # input stops a second time, since that is when opening is not enough
        back = stalls > 1 and revive(self.source)

        if stalls == 1:
            emit("listen_waiting", "the input stopped feeding, opening it again")
        elif back and stalls == 2:
            emit("listen_waiting", "the input stopped feeding, it was restarted")
        elif stalls == KEEPS_STALLING:
            emit("listen_waiting", "the input keeps stopping, try another usb port")

        self.stop_flag.wait(min(RETRY * stalls, RETRY_MAX))

    def _open(self):
        device = self.device
        if self.source or self.card:
            found = find_source(pipewire_sources(self.rate), self.source, self.card)
            if found is None:
                raise RuntimeError("the input %s is not there" % (self.source or self.card))

            # what is really being read, which the profile may have renamed
            self.source, self.card = found["id"], found.get("card", "")

            # the index moves when anything is plugged in, the name does not
            index, _ = alsa_device(PIPEWIRE)
            if index is None:
                raise RuntimeError("pipewire is on no alsa device list here")
            device = index

        aim(self.source)

        return sd.InputStream(
            device=device,
            channels=1,
            samplerate=self.rate,
            blocksize=pitch.HOP,
            dtype="float32",
            callback=self._callback,
        )

    def _watch(self, ended):
        """says when the source goes away under an open stream.

        the alsa plugin is pointed at a node by name and goes on answering
        silence when that node is destroyed, so a profile switched mid session
        is heard as a guitar nobody is playing and never as an error.
        """
        while not ended.wait(WATCH):
            if self.stop_flag.is_set():
                return

            names = [source["id"] for source in pipewire_sources(self.rate)]
            # an empty answer is pw-dump silent, not a machine with no inputs
            if names and self.source not in names:
                emit("listen_waiting", "the input went away, opening it again")
                self.lost.set()
                return

    def _loop(self):
        tracker = pitch.Tracker(self.rate)
        buffer = np.zeros(pitch.FRAME, dtype=np.float32)
        elapsed = 0.0
        last_level = -1.0

        self.lost.clear()
        self.frames = 0
        began = time.monotonic()
        ended = threading.Event()
        stream = self.stream

        try:
            stream.start()
            emit(
                "listening",
                data={
                    "device": self.device,
                    "source": self.source,
                    "card": self.card,
                    "rate": self.rate,
                },
            )

            if self.source:
                threading.Thread(target=self._watch, args=(ended,), daemon=True).start()

            while not self.stop_flag.is_set() and not self.lost.is_set():
                # an input that stopped and one dribbling in both end here
                if (time.monotonic() - began) - self.frames / self.rate > STALL:
                    return "stall"

                try:
                    block = self.blocks.get(timeout=0.2)
                except queue.Empty:
                    continue

                # a block slides the window, so a note is seen four times
                buffer = np.concatenate((buffer[len(block):], block))
                elapsed += len(block) / self.rate

                # the click goes out of a speaker and comes back in on an
                # open microphone, and it is a note nobody played
                tracker.deaf = ()
                if self.metronome is not None and self.metronome.sounding():
                    tracker.deaf = pitch.CLICK_PITCHES

                note = tracker.push(buffer, elapsed)
                if note is not None:
                    emit("note", data=note)

                if elapsed - last_level >= LEVEL_INTERVAL:
                    last_level = elapsed
                    freq, confidence, rms = pitch.detect(buffer, self.rate)
                    level = {"t": round(elapsed, 3), "rms": round(rms, 4)}
                    if freq > 0.0:
                        exact = pitch.midi_from_freq(freq)
                        midi = int(round(exact))
                        level.update(
                            {
                                "freq": round(freq, 2),
                                "midi": midi,
                                "name": pitch.note_name(midi),
                                "cents": round((exact - midi) * 100.0, 1),
                                "conf": round(confidence, 3),
                            }
                        )
                    emit("level", data=level)
        finally:
            ended.set()
            shut(stream)

        return "lost" if self.lost.is_set() else "stop"


def stop(listener):
    """shuts a stream and waits for it, since portaudio cannot be restarted
    with one open and the list of devices is read by restarting it."""
    if listener is None:
        return None

    listener.stop_flag.set()
    if not listener.done.wait(4.0):
        # portaudio will not start over with one open, and the list comes back short
        emit("log", "the input did not shut, the list may be short one device")
    return None


def main():
    listener = None

    # one metronome for the session, the same as the input: it holds an output
    # open while it counts and opening one per beat would be heard as a limp
    metronome = Metronome()
    threading.Thread(target=metronome.run, daemon=True).start()

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue

        try:
            command = json.loads(line)
        except ValueError:
            emit("error", "command is not json: %s" % line)
            continue

        action = command.get("action")

        if action == "devices":
            # the stream goes first, and the app opens it again when the list
            # gets there: portaudio will not start over with one open
            listener = stop(listener)
            try:
                emit("devices", data={"devices": list_devices()})
            except Exception as error:
                emit("error", str(error))

        elif action == "listen":
            listener = stop(listener)
            device = command.get("device")
            rate = int(command.get("rate") or 44100)
            listener = Listener(
                device,
                rate,
                command.get("source") or "",
                command.get("card") or "",
                metronome,
            )
            threading.Thread(target=listener.run, daemon=True).start()

        elif action == "stop":
            listener = stop(listener)

        elif action == "click":
            metronome.set(command.get("bpm"), command.get("beats"))

        elif action == "quit":
            break

        else:
            emit("error", "unknown action: %s" % action)

    metronome.stop_flag.set()
    metronome.set(0, 0)
    stop(listener)

    # portaudio asserts and dumps core on the way out when an open never
    # returned, and there is nothing left to tidy that is worth that
    os._exit(0)


if __name__ == "__main__":
    main()
