"""the wire reader, which is what stands in for the schemas librespot lacks."""

import spotify


def one_field(number, payload):
    """a length delimited field, the way protobuf lays it on the wire."""
    return bytes([number << 3 | 2, len(payload)]) + payload


def test_varint_reads_one_byte():
    assert spotify.varint(bytes([5]), 0) == (5, 1)


def test_varint_reads_the_bytes_that_carry_on():
    # 300 is two bytes, and a length over 127 is the ordinary case here
    assert spotify.varint(bytes([0xAC, 0x02]), 0) == (300, 2)


def test_walk_reads_a_message_field_by_field():
    blob = one_field(1, b"name") + bytes([2 << 3 | 0, 42])

    assert list(spotify.walk(blob)) == [(1, b"name"), (2, 42)]


def test_walk_stops_at_a_wire_type_it_cannot_read():
    # a fixed64 is not read here, and stopping beats answering nonsense
    blob = one_field(1, b"name") + bytes([2 << 3 | 1]) + b"\x00" * 8

    assert list(spotify.walk(blob)) == [(1, b"name")]


def test_field_answers_the_first_of_that_number():
    blob = one_field(1, b"first") + one_field(1, b"second")

    assert spotify.field(blob, 1) == b"first"


def test_field_answers_nothing_when_it_is_not_there():
    assert spotify.field(one_field(1, b"only"), 2) is None


def test_a_long_field_is_read_whole():
    # the attributes of a playlist run past the single byte length every time
    payload = b"x" * 300
    blob = bytes([1 << 3 | 2, 0xAC, 0x02]) + payload

    assert spotify.field(blob, 1) == payload
