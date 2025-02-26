package convert

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zibbp/spotify-playlist-sync/config"
	"github.com/zibbp/spotify-playlist-sync/db"
	"github.com/zibbp/spotify-playlist-sync/navidrome"
	"github.com/zibbp/spotify-playlist-sync/spotify"
	"github.com/zibbp/spotify-playlist-sync/tidal"
	libSpotify "github.com/zmb3/spotify/v2"
	"golang.org/x/exp/slices"

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

// SpotifyToTidal converts a user's Spotify playlists to Tidal playlists.
func (s *Service) SpotifyToTidal(ctx context.Context, saveMissingTracks bool, saveTidalPlaylist bool, saveNavidromePlaylist bool, spotifyPlaylistIDs []string) error {
	log.Info().Msg("Starting Spotify to Tidal conversion")

	// get all playlists from Spotify
	spotifyPlaylists, err := s.SpotifyService.GetUserPlaylists()
	if err != nil {
		return err
	}

	log.Info().Msgf("fetched %d Spotify playlists", len(spotifyPlaylists))

	tidalPlaylists, err := s.TidalService.GetUserPlaylists()
	if err != nil {
		return err
	}

	log.Info().Msgf("fetched %d Tidal playlists", len(tidalPlaylists.Items))

	// compare playlists
	for _, spotifyPlaylist := range spotifyPlaylists {
		if len(spotifyPlaylistIDs) > 0 {
			if !slices.Contains(spotifyPlaylistIDs, string(spotifyPlaylist.ID)) {
				log.Debug().Str("spotify_playlist_id", string(spotifyPlaylist.ID)).Str("spotify_playlist_name", spotifyPlaylist.Name).Msg("skipping playlist")
				continue
			}
		}

		// check if spotify playlist is in local database
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
			log.Warn().Str("name", spotifyPlaylist.Name).Str("id", spotifyPlaylist.ID.String()).Msgf("playlist is empty, skipping")
			continue
		}

		// get all local database tracks
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

		//
		// begin sync
		//

		// get all tracks from Spotify playlist
		spotifyTracks, err := s.SpotifyService.GetPlaylistTracks(spotifyPlaylist.ID)
		if err != nil {
			return err
		}

		log.Info().Str("platform", "spotify").Msgf("fetched %d tracks from playlist %s", len(spotifyTracks), spotifyPlaylist.Name)

		// get all tracks from Tidal playlist
		tidalTracks, err := s.TidalService.GetPlaylistTracks(tidalPlaylist.UUID)
		if err != nil {
			return err
		}

		log.Info().Str("platform", "tidal").Msgf("fetched %d tracks from playlist %s", len(tidalTracks.Items), tidalPlaylist.Title)

		// get the isrc of all tidal tracks
		tidalTrackISRCMap := make(map[string]bool)
		for _, tidalTrack := range tidalTracks.Items {
			tidalTrackISRCMap[tidalTrack.Isrc] = true
		}

		// hold missing tracks
		var missingTracks []*libSpotify.FullTrack

		// loop over each spotify track to convert
		for _, spotifyTrack := range spotifyTracks {
			if spotifyTrack == nil {
				log.Debug().Msg("skipping empty track")
				continue
			}

			// check if track is already in playlist using db
			if _, ok := dbPlaylistTrackMap[spotifyTrack.ID.String()]; ok {
				log.Debug().Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Msgf("track is already in playlist according to database")
				continue
			}

			// attempt to find track
			tidalTrack, err := s.spotifyToTidalTrack(ctx, spotifyTrack)
			if err != nil {
				log.Error().Err(err).Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Str("spotify_track_isrc", spotifyTrack.ExternalIDs["isrc"]).Msgf("failed to find track on Tidal")
				missingTracks = append(missingTracks, spotifyTrack)
				continue
			}

			if tidalTrack == nil {
				missingTracks = append(missingTracks, spotifyTrack)
				log.Warn().Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Msgf("track not found")
				continue
			}

			// add track to playlist
			log.Info().Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Str("tidal_playlist_id", tidalPlaylist.UUID).Str("tidal_track_id", tidalTrack.Id).Msgf("adding track to tidal playlist")
			err = s.TidalService.AddTrackToPlaylist(tidalPlaylist.UUID, tidalTrack.Id)
			if err != nil {
				log.Error().Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Str("tidal_playlist_id", tidalPlaylist.UUID).Str("tidal_track_id", tidalTrack.Id).Msgf("error adding track to playlist")
				continue
			}

			// add track to database
			err = s.Queries.AddTrackToPlaylist(ctx, db.AddTrackToPlaylistParams{
				PlaylistID: sql.NullString{String: dbPlaylist, Valid: true},
				TrackID:    sql.NullString{String: spotifyTrack.ID.String(), Valid: true},
			})
			if err != nil {
				log.Error().Str("spotify_track_id", spotifyTrack.ID.String()).Str("spotify_track_name", spotifyTrack.Name).Msgf("error adding track to database")
				continue
			}

			// sleep to prevent rate limiting
			time.Sleep(1 * time.Second)
		}

		// write missing tracks to file
		if saveMissingTracks && (len(missingTracks) > 0) {
			log.Info().Str("spotify_playlist", spotifyPlaylist.Name).Msgf("processing complete - found %d missing tracks", len(missingTracks))
			err := spotify.WriteMissingTracks(fmt.Sprintf("%s", spotifyPlaylist.ID), spotify.MissingTracks{
				Playlist: spotifyPlaylist,
				Tracks:   missingTracks,
			})
			if err != nil {
				return err
			}
		}

		if saveTidalPlaylist {
			// fetch fresh tidal playlist to save to disk
			tPlaylist, err := s.TidalService.GetPlaylist(tidalPlaylist.UUID)
			if err != nil {
				return err
			}
			tPlaylistTracks, err := s.TidalService.GetPlaylistTracks(tidalPlaylist.UUID)
			if err != nil {
				return err
			}
			for _, tidalTrack := range tPlaylistTracks.Items {
				tPlaylist.Tracks = append(tPlaylist.Tracks, tidalTrack)
			}
			err = tidal.WriteTidalPlaylist(fmt.Sprintf("%s", tPlaylist.UUID), tPlaylist)
			if err != nil {
				return err
			}
		}

		if saveNavidromePlaylist {
			tPlaylist, err := s.TidalService.GetPlaylist(tidalPlaylist.UUID)
			if err != nil {
				return err
			}

			tPlaylistTracks, err := s.TidalService.GetPlaylistTracks(tidalPlaylist.UUID)
			if err != nil {
				return err
			}
			for _, tidalTrack := range tPlaylistTracks.Items {
				tPlaylist.Tracks = append(tPlaylist.Tracks, tidalTrack)
			}

			navidromePlaylist := navidrome.Playlist{
				SourceId:      spotifyPlaylist.ID.String(),
				DestinationId: tidalPlaylist.UUID,
				Name:          spotifyPlaylist.Name,
				Description:   spotifyPlaylist.Description,
			}

			for _, track := range tPlaylist.Tracks {
				navidromePlaylist.Tracks = append(navidromePlaylist.Tracks, navidrome.Track{
					ID:       strconv.FormatInt(track.ID, 10),
					Title:    track.Title,
					Album:    track.Album.Title,
					Artist:   track.Artist.Name,
					Duration: track.Duration,
					ISRC:     track.Isrc,
				})
			}

			err = navidrome.WriteNavidromePlaylist(fmt.Sprintf("%s", tPlaylist.UUID), navidromePlaylist)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
