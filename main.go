package main

import (
	"context"
	"database/sql"
	_ "embed"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/zibbp/spotify-playlist-sync/config"
	"github.com/zibbp/spotify-playlist-sync/convert"
	"github.com/zibbp/spotify-playlist-sync/db"
	"github.com/zibbp/spotify-playlist-sync/spotify"
	"github.com/zibbp/spotify-playlist-sync/tidal"

	"github.com/urfave/cli/v2"
)

//go:embed schema.sql
var ddl string

func initialize() (*config.Config, *config.JsonConfigService, *spotify.Service, *db.Queries) {
	ctx := context.Background()

	// initialize config
	c, err := config.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// database
	dbConn, err := sql.Open("sqlite3", "/data/tracks.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}

	// create tables
	if _, err := dbConn.ExecContext(ctx, ddl); err != nil {
		log.Fatal().Err(err).Msg("Failed to create tables")
	}
	queries := db.New(dbConn)

	// load json config which has credentials
	jsonConfig := config.NewJsonConfigService("/data/config.json")
	err = jsonConfig.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load Spotify config")
	}

	// initialize the spotify connection
	spotifyService, err := spotify.Initialize(c.SpotifyClientId, c.SpotifyClientSecret, c.SpotifyRedirectUri, jsonConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Spotify service")
	}

	// authenticate with spotify
	err = spotifyService.Authenticate()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to authenticate with Spotify")
	}

	return c, jsonConfig, spotifyService, queries
}

func main() {
	var saveMissingTracks bool
	var saveTidalPlaylist bool
	var saveNavidromePlaylist bool

	app := &cli.App{
		Name:  "spotify-playlist-sync",
		Usage: "sync spotify playlists to other services",
		Commands: []*cli.Command{
			{
				Name:  "tidal",
				Usage: "sync playlists to tidal",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "save-missing-tracks",
						Usage:       "Save missing tracks during the conversion",
						Destination: &saveMissingTracks,
					},
					&cli.BoolFlag{
						Name:        "save-tidal-playlist",
						Usage:       "Save the tidal playlist",
						Destination: &saveTidalPlaylist,
					},
					&cli.BoolFlag{
						Name:        "save-navidrome-playlist",
						Usage:       "Save a version of the tidal playlist for importing in Navidrome",
						Destination: &saveNavidromePlaylist,
					},
					&cli.StringSliceFlag{
						Name:    "spotify-playlist-id",
						Aliases: []string{"spi"},
						Usage:   "List of Spotify playlist IDs to sync. Defaults to all user playlists if not provided.",
					},
				},
				Action: func(cCtx *cli.Context) error {
					c, jsonConfigService, spotifyService, queries := initialize()

					spotifyPlaylistIDs := cCtx.StringSlice("spotify-playlist-id")

					tidalService, err := tidal.Initialize(c.TidalClientId, c.TidalClientSecret, jsonConfigService)
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to initialize Tidal service")
					}

					// authenticate with Tidal
					err = tidalService.DeviceAuthenticate()
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to authenticate with Tidal")
					}

					// convert
					convertService, err := convert.Initialize(spotifyService, tidalService, jsonConfigService, queries)
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to initialize convert service")
					}

					err = convertService.SpotifyToTidal(saveMissingTracks, saveTidalPlaylist, saveMissingTracks, spotifyPlaylistIDs)
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to convert Spotify to Tidal")
					}

					return nil
				},
			},
		},
	}

	debug := os.Getenv("DEBUG")
	if debug == "true" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	container := os.Getenv("CONTAINER")
	if container != "true" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Failed to run app")
	}

}
