package spotify

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

var (
	ch    = make(chan *spotify.Client)
	state = "music-utils"
)

func (s *Service) authFlow() (*spotify.Client, error) {
	// Ensure Spotify application ID and secret are set
	if s.clientId == "" || s.clientSecret == "" {
		return nil, fmt.Errorf("spotify client ID and secret not provided")
	}

	// Check if Spotify access and refresh token is set
	// If set, fetch and return client
	if s.config.Get().Spotify.AccessToken == "" || s.config.Get().Spotify.RefreshToken == "" {
		log.Warn().Msg("Spotify access token and refresh token not set")
		client, err := s.auth()
		if err != nil {
			return nil, fmt.Errorf("error authenticating with Spotify: %w", err)
		}

		return client, nil
	}

	// Continue with auth flow
	// Use Spotify refresh token to get a new token and create client
	tok := &oauth2.Token{
		AccessToken:  s.config.Get().Spotify.AccessToken,
		RefreshToken: s.config.Get().Spotify.RefreshToken,
		Expiry:       s.config.Get().Spotify.Expiry,
		TokenType:    s.config.Get().Spotify.TokenType,
	}
	spotClientID := s.clientId
	spotClientSecret := s.clientSecret
	redirectURI := s.clientRedirectUri
	auth := spotifyauth.New(spotifyauth.WithClientID(spotClientID), spotifyauth.WithClientSecret(spotClientSecret), spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopePlaylistReadPrivate))

	client := spotify.New(auth.Client(context.Background(), tok))

	newTok, _ := client.Token()
	c := s.config.Get()
	c.Spotify.AccessToken = newTok.AccessToken
	c.Spotify.RefreshToken = newTok.RefreshToken
	c.Spotify.Expiry = newTok.Expiry
	c.Spotify.TokenType = newTok.TokenType

	err := s.config.Update(c)
	if err != nil {
		return nil, fmt.Errorf("error updating Spotify config: %w", err)
	}

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error getting current user: %w", err)
	}
	log.Info().Msgf("Spotify - logged in as: %s", user.ID)

	return client, nil
}

func (s *Service) auth() (*spotify.Client, error) {
	spotClientID := s.clientId
	spotClientSecret := s.clientSecret
	redirectURI := s.clientRedirectUri
	auth := spotifyauth.New(spotifyauth.WithClientID(spotClientID), spotifyauth.WithClientSecret(spotClientSecret), spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopePlaylistReadPrivate))
	// Start an HTTP server
	http.HandleFunc("/callback", s.completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	go func() {
		err := http.ListenAndServe(":28542", nil)
		if err != nil {
			log.Error().Msgf("Error starting HTTP server: %v", err)
		}
	}()

	url := auth.AuthURL(state)
	log.Info().Msgf("Please log in to Spotify by visiting the following page in your browser: %s", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error getting current user: %w", err)
	}
	log.Info().Msgf("Spotify - logged in as: %s", user.ID)
	return client, nil
}

func (s *Service) completeAuth(w http.ResponseWriter, r *http.Request) {
	spotClientID := s.clientId
	spotClientSecret := s.clientSecret
	redirectURI := s.clientRedirectUri
	auth := spotifyauth.New(spotifyauth.WithClientID(spotClientID), spotifyauth.WithClientSecret(spotClientSecret), spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopePlaylistReadPrivate))

	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Error().Msgf("Couldn't get token: %v", err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Error().Msgf("State mismatch: %s != %s\n", st, state)
	}

	// Save token to config

	c := s.config.Get()
	c.Spotify.AccessToken = tok.AccessToken
	c.Spotify.RefreshToken = tok.RefreshToken
	c.Spotify.Expiry = tok.Expiry
	c.Spotify.TokenType = tok.TokenType

	err = s.config.Update(c)
	if err != nil {
		log.Error().Err(err).Msg("Error updating Spotify config")
	}

	// use the token to get an authenticated client
	client := spotify.New(auth.Client(r.Context(), tok))
	ch <- client
}
