"""reads the spotify library, so a playlist can be looked at as practice.

the login is librespot and so is everything after it: the calls go to the
endpoint the desktop client itself talks to, over the session the login opened.
the public web api is not used at all, because its quota belongs to the client
id every librespot login shares and it answers 429 to all of us at once.
"""

import argparse
import json
import sys
import threading
import webbrowser

from librespot.core import MercuryRequests, OAuth, Session
from librespot.mercury import RawMercuryRequest
from librespot.metadata import PlaylistId, TrackId
from librespot.proto import ExtensionKind_pb2 as kinds
from librespot.proto import Playlist4External_pb2 as playlist4
from librespot.proto.ExtendedMetadata_pb2 import (
    BatchedEntityRequest,
    BatchedExtensionResponse,
    EntityRequest,
    ExtensionQuery,
)
from librespot.proto.Metadata_pb2 import Track
from requests.structures import CaseInsensitiveDict

# the login answers BadCredentials without streaming on the list
SCOPES = [
    "streaming",
    "user-read-email",
    "user-read-private",
    "user-library-read",
    "playlist-read-private",
    "playlist-read-collaborative",
]

# how many things one metadata request asks about. five hundred came back whole
BATCH = 300

# the id the liked songs are asked for by. spotify has no playlist for them,
# they are a collection of their own, and the interface should not have to know
LIKED = "liked"

PROTOBUF = CaseInsensitiveDict({"content-type": "application/x-protobuf"})


def emit(event, message="", data=None):
    line = {"event": event, "message": message}
    if data is not None:
        line["data"] = data
    sys.stdout.write(json.dumps(line) + "\n")
    sys.stdout.flush()


def login(credentials):
    port = 4381
    redirect = "http://127.0.0.1:%d/login" % port

    def open_browser(url):
        emit("spotify_log", "opening the login in your browser")
        try:
            webbrowser.open(url)
        except Exception:
            pass
        # the browser does not always open, and a login nobody can reach looks
        # like the app hanging
        threading.Timer(4.0, lambda: emit("spotify_log", url)).start()

    oauth = (
        OAuth(MercuryRequests.keymaster_client_id, redirect, open_browser)
        .set_scopes(SCOPES)
        .set_listen_all(True)
    )

    builder = Session.Builder()
    builder.conf.store_credentials = True
    builder.conf.stored_credentials_file = credentials
    builder.login_credentials = oauth.flow()
    builder.create()

    emit("spotify_ready", "logged in")


def session_of(credentials):
    """the session the login left behind, dialled again when an access point refuses."""
    for attempt in range(3):
        try:
            return Session.Builder().stored_file(credentials).create()
        except ConnectionRefusedError:
            if attempt == 2:
                raise


def varint(blob, at):
    """the number that starts at that byte, and where it ended."""
    value = shift = 0
    while at < len(blob):
        byte = blob[at]
        at += 1
        value |= (byte & 0x7F) << shift
        if not byte & 0x80:
            break
        shift += 7
    return value, at


def walk(blob):
    """the fields of a protobuf message as they lie on the wire, in order."""
    at = 0
    while at < len(blob):
        key, at = varint(blob, at)
        number, wire = key >> 3, key & 7
        if wire == 2:
            length, at = varint(blob, at)
            yield number, blob[at:at + length]
            at += length
        elif wire == 0:
            value, at = varint(blob, at)
            yield number, value
        else:
            return


def field(blob, number):
    """the first field of that number, and nothing when there is none."""
    for found, value in walk(blob):
        if found == number:
            return value
    return None


def batched(session, kind, uris):
    """one kind of metadata about many things, asked for a few hundred at a time."""
    found = {}

    for at in range(0, len(uris), BATCH):
        asked = [
            EntityRequest(entity_uri=uri, query=[ExtensionQuery(extension_kind=kind)])
            for uri in uris[at:at + BATCH]
        ]
        answer = session.api().send(
            "POST",
            "/extended-metadata/v0/extended-metadata",
            PROTOBUF,
            BatchedEntityRequest(entity_request=asked).SerializeToString(),
        )

        batch = BatchedExtensionResponse()
        batch.ParseFromString(answer.content)
        for group in batch.extended_metadata:
            for entity in group.extension_data:
                if entity.header.status_code == 200:
                    found[entity.entity_uri] = entity.extension_data.value

    return found


def rootlist(session):
    """the playlists of the account, in the order they are kept in."""
    answer = session.api().send(
        "GET", "/playlist/v2/user/%s/rootlist" % session.username(), None, None
    )

    content = playlist4.SelectedListContent()
    content.ParseFromString(answer.content)

    return [
        item.uri
        for item in content.contents.items
        if item.uri.startswith("spotify:playlist:")
    ]


def playlists(session):
    uris = rootlist(session)
    described = batched(session, kinds.LIST_METADATA, uris)

    found = [{"id": LIKED, "name": "Liked songs"}]
    for uri in uris:
        blob = described.get(uri)
        if blob is None:
            continue

        # librespot has no schema for that answer, and the name is in its first
        # field, which is the attributes message it does have one for
        attributes = playlist4.ListAttributes()
        attributes.ParseFromString(field(blob, 1) or b"")

        # a playlist its owner deleted stays on the rootlist with no name
        if attributes.name:
            found.append({"id": uri.split(":")[-1], "name": attributes.name})

    return found


def liked(session):
    """the track uris of the collection, which is what spotify calls the likes."""
    answer = session.mercury().send_sync(
        RawMercuryRequest.get(
            "hm://collection/collection/%s?allowonlytracks=true&complete=true"
            % session.username()
        )
    )

    uris = []
    for number, item in walk(answer.payload):
        if number != 1 or not isinstance(item, bytes):
            continue
        # every item of the collection carries the id of the track in its second
        gid = field(item, 2)
        if isinstance(gid, bytes):
            uris.append(TrackId.from_hex(gid.hex()).to_spotify_uri())

    return uris


def contents(session, playlist):
    """the track uris of one playlist, whichever of the two it is."""
    if playlist == LIKED:
        return liked(session)

    content = session.api().get_playlist(PlaylistId.from_base62(playlist))

    return [
        item.uri
        for item in content.contents.items
        if item.uri.startswith("spotify:track:")
    ]


def tracks(session, playlist):
    uris = contents(session, playlist)
    described = batched(session, kinds.TRACK_V4, uris)

    found = []
    for uri in uris:
        blob = described.get(uri)
        if blob is None:
            continue

        track = Track()
        track.ParseFromString(blob)

        # a local file comes back with no artist and cannot be looked up anywhere
        if track.name and track.artist:
            found.append({"artist": track.artist[0].name, "title": track.name})

    return found


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["login", "playlists", "tracks"])
    parser.add_argument("--credentials", required=True)
    parser.add_argument("--playlist")
    args = parser.parse_args()

    try:
        if args.action == "login":
            login(args.credentials)
            return 0

        session = session_of(args.credentials)

        if args.action == "playlists":
            emit("spotify_playlists", data={"playlists": playlists(session)})
            return 0

        if not args.playlist:
            emit("spotify_error", "no playlist was asked for")
            return 1

        emit("spotify_tracks", data={"tracks": tracks(session, args.playlist)})
        return 0

    except FileNotFoundError:
        emit("spotify_error", "not logged in yet, the spotify screen has the button")
        return 1
    except Exception as error:
        # the name of the class carries half of what librespot says went wrong
        emit("spotify_error", "%s: %s" % (type(error).__name__, error))
        return 1


if __name__ == "__main__":
    sys.exit(main())
