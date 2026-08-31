"""the live audio side: opens an input device and reports what is being played.

stays alive for the whole session and takes one json command per line on
stdin, because opening a device costs a fraction of a second and doing it on
every note would be heard as a gap.
"""

import json
import queue
import sys
import threading

import numpy as np
import sounddevice as sd

import pitch

# the tuner needs a reading even while the same note is held, and twenty a
# second is smooth to the eye without flooding the pipe with json
LEVEL_INTERVAL = 0.05


def emit(event, message="", data=None):
    line = {"event": event, "message": message}
    if data is not None:
        line["data"] = data
    sys.stdout.write(json.dumps(line) + "\n")
    sys.stdout.flush()


def list_devices():
    hosts = sd.query_hostapis()
    devices = []
    default = sd.default.device[0]

    for index, device in enumerate(sd.query_devices()):
        if device["max_input_channels"] < 1:
            continue
        devices.append(
            {
                "index": index,
                "name": device["name"],
                "host": hosts[device["hostapi"]]["name"],
                "channels": device["max_input_channels"],
                "rate": int(device["default_samplerate"]),
                "default": index == default,
            }
        )

    return devices


class Listener:
    """one open input stream, feeding the tracker frame by frame."""

    def __init__(self, device, rate):
        self.device = device
        self.rate = rate
        self.blocks = queue.Queue()
        self.stream = None
        self.stop_flag = threading.Event()

    def _callback(self, indata, frames, time_info, status):
        if status:
            # an overflow means the reader is late, and the audio it dropped is
            # gone. saying so beats reporting notes that were never played
            emit("audio_warning", str(status))
        # only the first channel: a stereo interface would otherwise average
        # the guitar with whatever is on the other input
        self.blocks.put(indata[:, 0].copy())

    def run(self):
        try:
            self.stream = sd.InputStream(
                device=self.device,
                channels=1,
                samplerate=self.rate,
                blocksize=pitch.HOP,
                dtype="float32",
                callback=self._callback,
            )
        except Exception as error:
            emit("listen_error", str(error))
            return

        tracker = pitch.Tracker(self.rate)
        buffer = np.zeros(pitch.FRAME, dtype=np.float32)
        elapsed = 0.0
        last_level = -1.0

        with self.stream:
            emit("listening", data={"device": self.device, "rate": self.rate})

            while not self.stop_flag.is_set():
                try:
                    block = self.blocks.get(timeout=0.2)
                except queue.Empty:
                    continue

                # the frame is four hops long, so each block slides the window
                # instead of replacing it and a note is seen by four frames
                buffer = np.concatenate((buffer[len(block):], block))
                elapsed += len(block) / self.rate

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

        emit("stopped")


def main():
    listener = None

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
            try:
                emit("devices", data={"devices": list_devices()})
            except Exception as error:
                emit("error", str(error))

        elif action == "listen":
            if listener is not None:
                listener.stop_flag.set()
            device = command.get("device")
            rate = int(command.get("rate") or 44100)
            listener = Listener(device, rate)
            threading.Thread(target=listener.run, daemon=True).start()

        elif action == "stop":
            if listener is not None:
                listener.stop_flag.set()
                listener = None

        elif action == "quit":
            break

        else:
            emit("error", "unknown action: %s" % action)

    if listener is not None:
        listener.stop_flag.set()


if __name__ == "__main__":
    main()
