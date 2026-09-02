"""tests for the way an input is found again, over lists built here."""

import worker


# the same pedal under the two profiles of one card, which is what a switch
# between duplex and pro audio does to the name of a node
CARD = "Jieli Technology USB Composite Device at usb-0000:00:14.0-4.4, full speed"
DUPLEX = "alsa_input.usb-Jieli_Technology_USB_Composite_Device-00.analog-stereo"
PRO = "alsa_input.usb-Jieli_Technology_USB_Composite_Device-00.pro-input-0"

WEBCAM = {
    "id": "alsa_input.usb-046d_C270_HD_WEBCAM-02.mono-fallback",
    "card": "C270 HD WEBCAM at usb-0000:00:14.0-10, high speed",
}


def sources(*names):
    return [WEBCAM] + [{"id": name, "card": CARD} for name in names]


def test_the_name_is_what_answers_first():
    found = worker.find_source(sources(DUPLEX, PRO), PRO, CARD)

    assert found["id"] == PRO


def test_the_card_answers_when_the_profile_renamed_the_node():
    found = worker.find_source(sources(DUPLEX), PRO, CARD)

    assert found["id"] == DUPLEX


# a config written before the card was kept has only the name, and the half of
# it that is not the profile is what is left to go on
def test_the_stem_of_the_name_answers_for_a_config_with_no_card():
    found = worker.find_source(sources(DUPLEX), PRO, "")

    assert found["id"] == DUPLEX


def test_a_name_of_one_part_is_not_a_stem():
    assert worker.stem("bluez_input.C8:24:78:11:D4:52") == ""
    assert worker.stem(DUPLEX) == worker.stem(PRO)


def test_a_card_that_is_gone_is_not_another_card():
    assert worker.find_source([WEBCAM], PRO, CARD) is None


def test_nothing_kept_is_nothing_found():
    assert worker.find_source(sources(DUPLEX), "", "") is None


def test_a_bluetooth_input_is_not_every_other_one():
    other = [{"id": "bluez_input.AA:BB:CC:DD:EE:FF", "card": "another headset"}]

    assert worker.find_source(other, "bluez_input.C8:24:78:11:D4:52", "") is None


def test_the_card_is_read_off_the_props_the_node_carries():
    assert worker.card_of({"api.alsa.card.longname": CARD, "node.name": PRO}) == CARD
    assert worker.card_of({"node.name": PRO}) == PRO
    assert worker.card_of({}) == ""
