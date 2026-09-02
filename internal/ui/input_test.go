package ui

import (
	"encoding/json"
	"testing"

	"github.com/VitorCdSouza/fretdeck/internal/bridge"
	"github.com/VitorCdSouza/fretdeck/internal/config"
)

// the same pedal under the two profiles of one card. Switching a card between
// duplex and pro audio renames the node, and the name that was saved is on no
// list until somebody switches it back, which is what used to lose the input.
const (
	card   = "Jieli Technology USB Composite Device at usb-0000:00:14.0-4.4, full speed"
	duplex = "alsa_input.usb-Jieli_Technology_USB_Composite_Device-00.analog-stereo"
	pro    = "alsa_input.usb-Jieli_Technology_USB_Composite_Device-00.pro-input-0"
)

func withInputs(t *testing.T, cfg config.Config, ids ...string) *Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := &Model{cfg: cfg}
	m.devices = []deviceInfo{{ID: "alsa_input.usb-046d_C270-02.mono-fallback",
		Card: "C270 HD WEBCAM", Name: "webcam", Rate: 44100}}
	for _, id := range ids {
		m.devices = append(m.devices, deviceInfo{ID: id, Card: card, Name: "pedal", Rate: 44100})
	}
	return m
}

func TestTheCardFindsTheInputAProfileChangeRenamed(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100}, duplex)

	found, ok := m.haveDevice()
	if !ok {
		t.Fatal("the pedal is on the list under the other profile and was not found")
	}
	if found.ID != duplex {
		t.Fatalf("want the input the card is on, got %q", found.ID)
	}
}

// a card can carry two inputs, and the one that was chosen is not the other
// one for as long as it is on the list under its own name.
func TestTheNameWinsOverTheCardWhileItIsThere(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100}, duplex, pro)

	found, ok := m.haveDevice()
	if !ok || found.ID != pro {
		t.Fatalf("want the name that was kept, got %q (%v)", found.ID, ok)
	}
	if m.chosen(m.devices[1]) {
		t.Fatal("the other input of the card is marked as the one in use")
	}
}

// a config kept before the card was written down has only the name, and the
// half of it that is not the profile is what is left to go on.
func TestTheStemOfTheNameAnswersForAConfigWithNoCard(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Rate: 44100}, duplex)

	found, ok := m.haveDevice()
	if !ok || found.ID != duplex {
		t.Fatalf("want the same card under its other profile, got %q (%v)", found.ID, ok)
	}
}

func TestABluetoothInputIsNotEveryOtherOne(t *testing.T) {
	if stem("bluez_input.C8:24:78:11:D4:52") != "" {
		t.Fatal("a name of one part answered for a stem")
	}
	if stem(duplex) != stem(pro) {
		t.Fatalf("the two profiles of one card differ: %q and %q", stem(duplex), stem(pro))
	}
}

func TestAnInputThatIsGoneIsNotAnotherCard(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100})

	if _, ok := m.haveDevice(); ok {
		t.Fatal("the webcam answered for a pedal that is not plugged in")
	}
}

// what the worker opened is what gets kept, or the next run goes looking for
// the name of a profile that is not on any more.
func TestTheNameTheWorkerOpenedIsKept(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100}, duplex)

	data, err := json.Marshal(listeningPayload{Source: duplex, Card: card, Rate: 44100})
	if err != nil {
		t.Fatal(err)
	}
	m.handle(bridge.Event{Event: bridge.EventListening, Data: data})

	if m.cfg.Source != duplex || m.cfg.Card != card {
		t.Fatalf("the config still says %q on %q", m.cfg.Source, m.cfg.Card)
	}
	if config.Load().Source != duplex {
		t.Fatalf("the input was not written down, the file says %q", config.Load().Source)
	}
	if m.fail != "" {
		t.Fatalf("listening is not a failure, the bar says %q", m.fail)
	}
}

// an input that is not there yet is being waited for, and a message on the
// error line would have somebody pressing r at a worker that is already busy.
func TestWaitingForAnInputIsNotAnError(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100})

	m.handle(bridge.Event{Event: bridge.EventListenWaiting, Message: "the input is not there"})

	if m.fail != "" {
		t.Fatalf("want the status line, the error line says %q", m.fail)
	}
	if m.status != "the input is not there" {
		t.Fatalf("want what is going on, the status says %q", m.status)
	}
}

// the input being off when the app opens is not a dead end: the answer that
// was kept stays kept, and the worker is left waiting for it by name.
func TestAnInputThatIsNotThereYetIsWaitedFor(t *testing.T) {
	m := withInputs(t, config.Config{Source: pro, Card: card, Rate: 44100})

	devices, err := json.Marshal(devicesPayload{Devices: m.devices})
	if err != nil {
		t.Fatal(err)
	}
	if cmd := m.handle(bridge.Event{Event: bridge.EventDevices, Data: devices}); cmd == nil {
		t.Fatal("want the worker asked to listen for it, got nothing to run")
	}

	if m.cfg.Source != pro || m.cfg.Card != card {
		t.Fatalf("the answer that was kept was thrown away, config says %q", m.cfg.Source)
	}
	if m.screen != screenConfig {
		t.Fatalf("want the config screen open on it, got %s", screenNames[m.screen])
	}
}
