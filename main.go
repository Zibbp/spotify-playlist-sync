package main

import (
	"context"
	"database/sql"
	_ "embed"
	"os"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/zibbp/spotify-playlist-convert/config"
	"github.com/zibbp/spotify-playlist-convert/convert"
	"github.com/zibbp/spotify-playlist-convert/db"
	"github.com/zibbp/spotify-playlist-convert/spotify"
	"github.com/zibbp/spotify-playlist-convert/tidal"

	"github.com/urfave/cli/v2"
)

//go:embed schema.sql
var ddl string

func initialize() (*config.Config, *config.JsonConfigService, *spotify.Service, *db.Queries) {
	c, err := config.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// database
	dbConn, err := sql.Open("sqlite3", "/data/tracks.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}

	ctx := context.Background()

	// create tables
	if _, err := dbConn.ExecContext(ctx, ddl); err != nil {
		log.Fatal().Err(err).Msg("Failed to create tables")
	}

	queries := db.New(dbConn)

	jsonConfig := config.NewJsonConfigService("/data/config.json")
	err = jsonConfig.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load Spotify config")
	}

	spotifyService, err := spotify.Initialize(c.SpotifyClientId, c.SpotifyClientSecret, c.SpotifyRedirectUri, jsonConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Spotify service")
	}

	err = spotifyService.Authenticate()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to authenticate with Spotify")
	}

	return c, jsonConfig, spotifyService, queries
}

func main() {
	app := &cli.App{
		Name:  "spotify-convert",
		Usage: "convert spotify playlists to other services",
		Commands: []*cli.Command{
			{
				Name:  "tidal",
				Usage: "convert playlists to tidal",
				Action: func(cCtx *cli.Context) error {
					c, jsonConfigService, spotifyService, queries := initialize()

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

					err = convertService.SpotifyToTidal()
					if err != nil {
						log.Fatal().Err(err).Msg("Failed to convert Spotify to Tidal")
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Failed to run app")
	}

}
