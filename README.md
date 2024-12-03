# Spotify Playlist Sync

Sync Spotify playlists to different services. Currently only supports Tidal.

A local database is used to speed up subsequent runs, skipping any tracks that have already been synced.

## Tidal

The Tidal conversion uses a mix of Tidal's [work-in-progress public APIs](https://developer.tidal.com/apiref?spec=catalogue-v2&ref=get-albums-v2) and some undocumented ones. 

### Sync

The track sync first checks if the Spotify track exists on Tidal by searching for the ISRC. If the track is not found by ISRC then a more crude method is used, searching for track name, arists, album, and duration.

Tidal has aggressive rate limits so a one-second sleep runs after every conversion. Subsequent runs should be much faster as the sync checks the local database first.

## Usage

### Requirements

- A Spotify [developer application](https://developer.spotify.com/) is required for the client ID and client secret.
- A Tidal [developer application](https://developer.tidal.com) is required for the client ID and client secret if you are converting to Tidal.

### Commands

Run the application with `-h` to see a list of commands. Currently only a `tidal` command exists which converts your Spotifyp playlists to Tidal playlists.

```bash
docker run --rm ghcr.io/zibbp/spotify-playlist-sync:latest -h
```

#### Tidal

Options

```bash
   --save-missing-tracks      Save missing tracks during the conversion (default: false)
   --save-tidal-playlist      Save the tidal playlist (default: false)
   --save-navidrome-playlist  Save a version of the tidal playlist for importing in Navidrome (default: false)
```

- Save missing tracks writes all missing Spotify tracks to `/data/missing/<spotify_playlist_id>.json`.
- Save Tidal playlist writes the Tidal playlist to `/data/tidal/<tidal_playlist_id>.json`.
- Save Navidrome playlist writes the Tidal playlist in a special format for [importing into Navidrome](https://github.com/Zibbp/navidrome-utils).
   - Note that is not supported yet. It requires the `isrc` to be avilable in Navidrome's database which [is a work-in-progres](https://github.com/navidrome/navidrome/pull/2709).

### Docker

Docker is the recommended way to run the application. See [compose.yml](compose.yml) to get started.

- Modify the `command` to run whichever command and arguments.
- Update the various `*_CLIENT_ID` and `*_CLIENT_SECRET` variables with your values. 
- Update the `SPOTIFY_CLIENT_REDIRECT_URI` with the IP/hostname of your server.


## Development

Create a `.env` file with the below variables. Then use [task](https://taskfile.dev/) to run with `task dev -- tidal`.

```
SPOTIFY_CLIENT_ID=123
SPOTIFY_CLIENT_SECRET=123
TIDAL_CLIENT_ID=123
TIDAL_CLIENT_SECRET=123=
```
