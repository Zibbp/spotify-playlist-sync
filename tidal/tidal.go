package tidal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"
	"github.com/zibbp/spotify-playlist-sync/config"
	tidal_search "github.com/zibbp/spotify-playlist-sync/tidal/search"
	tidal_tracks "github.com/zibbp/spotify-playlist-sync/tidal/tracks"
	"golang.org/x/time/rate"

	"github.com/rs/zerolog/log"
)

//go:generate oapi-codegen --config=cfg-tracks.yaml https://developer.tidal.com/apiref/api-specifications/api-public-catalogue-jsonapi/tidal-catalog-v2-openapi-3.0.json
//go:generate oapi-codegen --config=cfg-search.yaml https://developer.tidal.com/apiref/api-specifications/api-public-search-jsonapi/tidal-search-v2-openapi-3.0.json

var (
	apiURL       = "https://api.tidal.com/v1"
	apiURL2      = "https://listen.tidal.com/v2"
	openAPIURL   = "https://openapi.tidal.com"
	openAPIv2URL = "https://openapi.tidal.com/v2"
)

type Service struct {
	ClientId          string
	ClientSecret      string
	AccessToken       string // device-flow user resources access token
	ClientAccessToken string // application client for accessing Tidal API non-user resources
	UserID            string
	Config            *config.JsonConfigService
	TracksApiClient   *tidal_tracks.ClientWithResponses
	SearchApiClient   *tidal_search.ClientWithResponses
}

// Rate limiter: Allow 5 requests per second with bursts of 2
var limiter = rate.NewLimiter(5, 2)

// Wraps an HTTP request with rate limiting
func rateLimitedDo(client *http.Client, req *http.Request) (*http.Response, error) {
	err := limiter.Wait(context.Background()) // Wait for rate limit token
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// Custom retry policy to handle 429 Too Many Requests
func retryPolicy(_ context.Context, resp *http.Response, err error) (bool, error) {
	if err != nil {
		return true, err // Retry on network errors
	}
	if resp.StatusCode == 429 { // Handle 429 Too Many Requests
		log.Info().Msg("rate limited by Tidal API. waiting before retrying.")
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if delay, err := strconv.Atoi(retryAfter); err == nil {
				time.Sleep(time.Duration(delay) * time.Second) // Wait based on Retry-After header
			}
		} else {
			time.Sleep(3 * time.Second)
		}
		return true, nil
	}
	if resp.StatusCode >= 500 { // Retry on 5xx errors
		return true, nil
	}
	return false, nil
}

func Initialize(clientId, clientSecret string, config *config.JsonConfigService) (*Service, error) {
	var s Service
	s.ClientId = clientId
	s.ClientSecret = clientSecret
	s.Config = config

	if s.Config.Get().Tidal.AccessToken != "" {
		s.AccessToken = s.Config.Get().Tidal.AccessToken
	}

	// client auth is non-interactive so run always
	clientAccessToken, err := s.clientAuth(s.ClientId, s.ClientSecret)
	if err != nil {
		return nil, err
	}

	s.ClientAccessToken = clientAccessToken

	bearerTokenAuth, err := securityprovider.NewSecurityProviderBearerToken(s.ClientAccessToken)
	if err != nil {
		return nil, err
	}

	// Configure retryable HTTP client
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5                          // Maximum retry attempts
	retryClient.RetryWaitMin = 500 * time.Millisecond // Minimum wait before retry
	retryClient.RetryWaitMax = 2 * time.Second        // Maximum wait before retry
	retryClient.CheckRetry = retryPolicy
	retryClient.Logger = nil // Disable logging

	// Convert retryablehttp.Client to standard http.Client
	client := retryClient.StandardClient()

	apiClient, err := tidal_tracks.NewClientWithResponses(openAPIv2URL, tidal_tracks.WithHTTPClient(client), tidal_tracks.WithRequestEditorFn(bearerTokenAuth.Intercept))
	if err != nil {
		return nil, err
	}

	searchApiClient, err := tidal_search.NewClientWithResponses(openAPIv2URL, tidal_search.WithHTTPClient(client), tidal_search.WithRequestEditorFn(bearerTokenAuth.Intercept))

	s.TracksApiClient = apiClient
	s.SearchApiClient = searchApiClient

	return &s, nil
}

// Perform device authentication with Tidal to access user resources
// Once the official Tidal API supports user authentication, this method will be updated
func (s *Service) DeviceAuthenticate() error {
	if s.Config.Get().Tidal.AccessToken == "" || s.Config.Get().Tidal.RefreshToken == "" {
		log.Debug().Msg("No Tidal access token found")

		deviceCode, err := s.getDeviceCode()
		if err != nil {
			return err
		}

		log.Info().Msgf("Please visit the following URL to authorize this application: https://%v", deviceCode.VerificationURIComplete)

		// start poll for authorization
		for {
			loginResponse, err := s.tokenLogin(*deviceCode)
			if err != nil {
				// continue polling
				log.Debug().Msg("Failed to login with Tidal")

			}

			if (loginResponse != nil && AuthLogin{} == loginResponse.AuthLogin) {
				if loginResponse.AuthError.Error == "expired_token" {
					log.Fatal().Msg("Tidal auth failed - device code expired")
				}
			} else {
				s.Config.JsonConfig.Tidal.UserID = strconv.Itoa(int(loginResponse.AuthLogin.User.UserID))
				s.Config.JsonConfig.Tidal.AccessToken = loginResponse.AuthLogin.AccessToken
				s.Config.JsonConfig.Tidal.RefreshToken = loginResponse.AuthLogin.RefreshToken
				s.Config.Save()
				break
			}

			d := time.Duration(deviceCode.Interval) * time.Second
			log.Debug().Msgf("Waiting %d seconds before trying again.", deviceCode.Interval)
			time.Sleep(d)

		}
	} else {
		log.Debug().Msg("Tidal access token found")
		_, err := s.checkSession(s.Config.Get().Tidal.AccessToken)
		if err != nil {
			// failed probably need to refresh
			log.Debug().Msg("Tidal access token expired")
			refresh, err := s.refreshSession(s.Config.Get().Tidal.RefreshToken)
			if err != nil {
				return err
			}

			s.Config.JsonConfig.Tidal.AccessToken = refresh.AccessToken
			s.Config.Save()

		}

		log.Debug().Msg("Tidal access token valid")
	}

	s.AccessToken = s.Config.Get().Tidal.AccessToken
	s.UserID = s.Config.Get().Tidal.UserID

	return nil
}

func (s *Service) GetTrackByISRCv2(ctx context.Context, isrc string) (*tidal_tracks.TracksResource, error) {

	resp, err := s.TracksApiClient.GetTracksWithResponse(ctx, &tidal_tracks.GetTracksParams{CountryCode: "US", FilterIsrc: &[]string{isrc}})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		log.Error().Str("response", string(resp.Body)).Msg("failed to get tracks")
		return nil, fmt.Errorf("failed to get tracks: %s", resp.Status())
	}

	tracks := *resp.ApplicationvndApiJSON200

	if *tracks.Data == nil || len(*tracks.Data) == 0 {
		return nil, fmt.Errorf("track not found")
	}

	return &(*tracks.Data)[0], nil
}

func (s *Service) SearchTrackv2(ctx context.Context, query string, country string) (*[]tidal_tracks.TracksResource, error) {
	resp, err := s.SearchApiClient.GetSearchResultsTracksRelationshipWithResponse(ctx, query, &tidal_search.GetSearchResultsTracksRelationshipParams{CountryCode: country, Include: &[]string{"tracks"}})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to search tracks: %s", resp.Status())
	}

	tracks := *resp.ApplicationvndApiJSON200

	if *tracks.Data == nil || len(*tracks.Data) == 0 {
		return nil, fmt.Errorf("track not found")
	}

	var responseTracks []tidal_tracks.TracksResource

	max := 5
	if len(*tracks.Data) < max {
		max = len(*tracks.Data)
	}

	for i := 0; i < max; i++ {
		trackId := (*tracks.Data)[i].Id
		// Get track
		trackResp, err := s.TracksApiClient.GetTracksWithResponse(ctx, &tidal_tracks.GetTracksParams{CountryCode: "US", FilterId: &[]string{trackId}})
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to search tracks: %s", resp.Status())
		}

		respData := *trackResp.ApplicationvndApiJSON200
		if *respData.Data == nil || len(*respData.Data) == 0 {
			log.Warn().Str("track_id", trackId).Msg("track not found")
			continue
		}

		responseTracks = append(responseTracks, (*respData.Data)[0])
	}

	return &responseTracks, nil
}
