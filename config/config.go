package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Debug               bool   `env:"DEBUG, default=false"`
	SpotifyClientId     string `env:"SPOTIFY_CLIENT_ID, required"`
	SpotifyClientSecret string `env:"SPOTIFY_CLIENT_SECRET, required"`
	SpotifyRedirectUri  string `env:"SPOTIFY_CLIENT_REDIRECT_URI, default=http://localhost:28542/callback"`
	TidalClientId       string `env:"TIDAL_CLIENT_ID, required"`
	TidalClientSecret   string `env:"TIDAL_CLIENT_SECRET, required"`
}

func Init() (*Config, error) {
	ctx := context.Background()

	var c Config
	if err := envconfig.Process(ctx, &c); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &c, nil
}
