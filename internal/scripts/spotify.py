"""reads the spotify library, so a playlist can be looked at as practice.

the login is librespot, which is also what gives the access token: no app has
to be registered for this, and no client id or secret is asked for anywhere.
the calls themselves go to the ordinary web api over urllib, since a playlist
is plain json and pulling in a client for it would be a dependency for nothing.
"""

import argparse
import json
import sys
import threading
import urllib.error
import urllib.parse
import urllib.request
import webbrowser

from librespot.core import MercuryRequests, OAuth, Session

API = "https://api.spotify.com/v1"

SCOPES = [
    "user-library-read",
    "playlist-read-private",
    "playlist-read-collaborative",
]

# the id the liked songs are asked for by. spotify has no playlist for them,
# they are their own endpoint, and the interface should not have to know that
LIKED = "liked"


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
    return Session.Builder().stored_file(credentials).create()


def token_of(session):
    return session.tokens().get_token(*SCOPES).access_token


def call(token, path, params=None):
    url = API + path
    if params:
        url += "?" + urllib.parse.urlencode(params)

    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.urlopen(request, timeout=20) as response:
        return json.loads(response.read().decode("utf-8"))


def pages(token, path, params, limit):
    """walks an endpoint that answers in pages, which all of these do."""
    offset = 0
    while True:
        page = call(token, path, dict(params, limit=limit, offset=offset))
        items = page.get("items", [])
        for item in items:
            yield item
        offset += len(items)
        if not items or offset >= page.get("total", 0):
            return


def playlists(token):
    found = [{"id": LIKED, "name": "Liked songs", "count": liked_count(token)}]

    for item in pages(token, "/me/playlists", {}, 50):
        found.append(
            {
                "id": item["id"],
                "name": item["name"],
                "count": item.get("tracks", {}).get("total", 0),
            }
        )

    return found


def liked_count(token):
    return call(token, "/me/tracks", {"limit": 1}).get("total", 0)


def tracks(token, playlist):
    if playlist == LIKED:
        path, params = "/me/tracks", {}
    else:
        path, params = "/playlists/%s/tracks" % playlist, {}

    found = []
    for item in pages(token, path, params, 50):
        track = item.get("track") or {}
        # a local file and an episode both come back in a playlist with no
        # artist list, and neither can be looked up anywhere
        artists = track.get("artists") or []
        if not artists or not track.get("name"):
            continue
        found.append({"artist": artists[0]["name"], "title": track["name"]})

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
        token = token_of(session)

        if args.action == "playlists":
            emit("spotify_playlists", data={"playlists": playlists(token)})
            return 0

        if not args.playlist:
            emit("spotify_error", "no playlist was asked for")
            return 1

        emit("spotify_tracks", data={"tracks": tracks(token, args.playlist)})
        return 0

    except FileNotFoundError:
        emit("spotify_error", "not logged in yet, connect spotify on the setup screen")
        return 1
    except urllib.error.HTTPError as error:
        emit("spotify_error", "spotify answered %d" % error.code)
        return 1
    except Exception as error:
        emit("spotify_error", str(error))
        return 1


if __name__ == "__main__":
    sys.exit(main())
