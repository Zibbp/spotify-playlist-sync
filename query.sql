-- name: GetTrackById :one
SELECT * FROM tracks
WHERE id = ? LIMIT 1;

-- name: AddTrackToPlaylist :exec
INSERT INTO playlist_tracks (playlist_id, track_id)
VALUES (?, ?);

-- name: GetPlaylistTracks :many
SELECT * FROM playlist_tracks
WHERE playlist_id = ?;

-- name: GetPlaylistById :one
SELECT * FROM playlists
WHERE id = ? LIMIT 1;

-- name: CreatePlaylist :one
INSERT INTO playlists (id)
VALUES (?)
RETURNING *;
