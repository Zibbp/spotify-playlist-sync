package convert

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zibbp/spotify-playlist-convert/config"
	"github.com/zibbp/spotify-playlist-convert/db"
	"github.com/zibbp/spotify-playlist-convert/spotify"
	"github.com/zibbp/spotify-playlist-convert/tidal"

	"github.com/rs/zerolog/log"
)

type Service struct {
	SpotifyService *spotify.Service
	TidalService   *tidal.Service
	Config         *config.JsonConfigService
	Queries        *db.Queries
}

func Initialize(spotifyService *spotify.Service, tidalService *tidal.Service, config *config.JsonConfigService, queries *db.Queries) (*Service, error) {
	var s Service
	s.SpotifyService = spotifyService
	s.TidalService = tidalService
	s.Config = config
	s.Queries = queries

	return &s, nil
}

func (s *Service) SpotifyToTidal() error {
	log.Info().Msg("Starting Spotify to Tidal conversion")

	// get all playlists from Spotify
	spotifyPlaylists, err := s.SpotifyService.GetUserPlaylists()
	if err != nil {
		return err
	}

	tidalPlaylists, err := s.TidalService.GetUserPlaylists()
	if err != nil {
		return err
	}

	// compare playlists
	for _, spotifyPlaylist := range spotifyPlaylists {

		// database stuff
		ctx := context.Background()
		dbPlaylist, err := s.Queries.GetPlaylistById(ctx, string(spotifyPlaylist.ID))
		if err == sql.ErrNoRows {
			// create new playlist
			_, err := s.Queries.CreatePlaylist(ctx, string(spotifyPlaylist.ID))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if dbPlaylist == "" {
			return fmt.Errorf("playlist is empty: %s", dbPlaylist)
		}

		dbPlaylistTracks, err := s.Queries.GetPlaylistTracks(ctx, sql.NullString{String: dbPlaylist, Valid: true})
		if err != nil {
			return err
		}

		// create map of tracks
		dbPlaylistTrackMap := make(map[string]bool)
		for _, dbPlaylistTrack := range dbPlaylistTracks {
			dbPlaylistTrackMap[dbPlaylistTrack.TrackID.String] = true
		}

		// check if spotify playlist id exists in tidal description
		// if it does not exist, create a new playlist
		found := false
		tidalPlaylist := tidal.Playlist{}
		for _, tidtidalPlaylist := range tidalPlaylists.Items {
			if strings.Contains(tidtidalPlaylist.Description, string(spotifyPlaylist.ID)) {
				found = true
				tidalPlaylist = tidtidalPlaylist
				break
			}
		}

		if !found {
			// create new playlist
			var playlistName string
			if spotifyPlaylist.Name == "" {
				playlistName = "Untitled"
			} else {
				playlistName = spotifyPlaylist.Name
			}
			log.Info().Msgf("Creating playlist: %s - %s", spotifyPlaylist.Name, spotifyPlaylist.Description)
			createdTidalPlaylist, err := s.TidalService.CreatePlaylist(playlistName, fmt.Sprintf("%s:%s", string(spotifyPlaylist.ID), spotifyPlaylist.Description))
			if err != nil {
				return err
			}

			tidalPlaylist = *createdTidalPlaylist
		}

		// check if playlist needs to be updated
		if tidalPlaylist.UUID != "" && (tidalPlaylist.Title != spotifyPlaylist.Name && spotifyPlaylist.Name != "") || tidalPlaylist.Description != fmt.Sprintf("%s:%s", string(spotifyPlaylist.ID), spotifyPlaylist.Description) {
			log.Info().Msgf("Updating playlist: %s - %s", spotifyPlaylist.Name, spotifyPlaylist.Description)
			err := s.TidalService.UpdatePlaylist(tidalPlaylist.UUID, spotifyPlaylist.Name, fmt.Sprintf("%s:%s", string(spotifyPlaylist.ID), spotifyPlaylist.Description))
			if err != nil {
				return err
			}
		}

		// begin sync

		// get all tracks from Spotify playlist
		spotifyTracks, err := s.SpotifyService.GetPlaylistTracks(spotifyPlaylist.ID)
		if err != nil {
			return err
		}

		// get all tracks from Tidal playlist
		tidalTracks, err := s.TidalService.GetPlaylistTracks(tidalPlaylist.UUID)
		if err != nil {
			return err
		}

		tidalTrackISRCMap := make(map[string]bool)

		for _, tidalTrack := range tidalTracks.Items {
			tidalTrackISRCMap[tidalTrack.Isrc] = true
		}

		for _, spotifyTrack := range spotifyTracks {
			// check if track is already in playlist using db
			if _, ok := dbPlaylistTrackMap[spotifyTrack.ID.String()]; ok {
				// log.Info().Msgf("Track already in playlist: %s - %s", spotifyTrack.ID, spotifyTrack.Name)
				continue
			}

			// attempt to find track
			tidalTrack, err := s.spotifyToTidalTrack(spotifyTrack)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to find track: %s - %s", spotifyTrack.ID, spotifyTrack.Name)
				continue
			}

			if tidalTrack == nil {
				log.Warn().Msgf("Track not found: %s - %s", spotifyTrack.ID, spotifyTrack.Name)
				continue
			}

			// add track to playlist
			log.Info().Msgf("Adding track to playlist: %s - %s from: %s", spotifyTrack.ID, spotifyTrack.Name, spotifyPlaylist.Name)
			err = s.TidalService.AddTrackToPlaylist(tidalPlaylist.UUID, tidalTrack.ID)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to add track to playlist: %s - %s", spotifyTrack.ID, spotifyTrack.Name)
				continue
			}

			// add track to database
			err = s.Queries.AddTrackToPlaylist(ctx, db.AddTrackToPlaylistParams{
				PlaylistID: sql.NullString{String: dbPlaylist, Valid: true},
				TrackID:    sql.NullString{String: spotifyTrack.ID.String(), Valid: true},
			})
			if err != nil {
				log.Error().Err(err).Msgf("Failed to add track to database: %s - %s", spotifyTrack.ID, spotifyTrack.Name)
				continue
			}

			time.Sleep(1 * time.Second)
		}

	}

	return nil
}
