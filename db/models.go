// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package db

import (
	"database/sql"
)

type Playlist struct {
	ID string
}

type PlaylistTrack struct {
	PlaylistID sql.NullString
	TrackID    sql.NullString
	AddedAt    sql.NullTime
}

type Track struct {
	ID string
}