package tidal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zibbp/spotify-playlist-convert/config"

	"github.com/rs/zerolog/log"
)

var (
	apiURL     = "https://api.tidal.com/v1"
	apiURL2    = "https://listen.tidal.com/v2"
	openAPIURL = "https://openapi.tidal.com"
)

type Service struct {
	ClientId          string
	ClientSecret      string
	AccessToken       string // device-flow user resources access token
	ClientAccessToken string // application client for accessing Tidal API non-user resources
	UserID            string
	Config            *config.JsonConfigService
}

type OpenAPITrackResource struct {
	ArtifactType string `json:"artifactType,omitempty"`
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	Artists      []struct {
		ID      string `json:"id,omitempty"`
		Name    string `json:"name,omitempty"`
		Picture []struct {
			URL    string `json:"url,omitempty"`
			Width  int    `json:"width,omitempty"`
			Height int    `json:"height,omitempty"`
		} `json:"picture,omitempty"`
		Main bool `json:"main,omitempty"`
	} `json:"artists,omitempty"`
	Album struct {
		ID         string `json:"id,omitempty"`
		Title      string `json:"title,omitempty"`
		ImageCover []struct {
			URL    string `json:"url,omitempty"`
			Width  int    `json:"width,omitempty"`
			Height int    `json:"height,omitempty"`
		} `json:"imageCover,omitempty"`
		VideoCover []any `json:"videoCover,omitempty"`
	} `json:"album,omitempty"`
	Duration      int    `json:"duration,omitempty"`
	TrackNumber   int    `json:"trackNumber,omitempty"`
	VolumeNumber  int    `json:"volumeNumber,omitempty"`
	Isrc          string `json:"isrc,omitempty"`
	Copyright     string `json:"copyright,omitempty"`
	MediaMetadata struct {
		Tags []string `json:"tags,omitempty"`
	} `json:"mediaMetadata,omitempty"`
	Properties struct {
	} `json:"properties,omitempty"`
	TidalURL string `json:"tidalUrl,omitempty"`
}

type OpenAPIISRCTrackSearch struct {
	Data []struct {
		Resource OpenAPITrackResource `json:"resource,omitempty"`
		ID       string               `json:"id,omitempty"`
		Status   int                  `json:"status,omitempty"`
		Message  string               `json:"message,omitempty"`
	} `json:"data,omitempty"`
	Metadata struct {
		Requested int `json:"requested,omitempty"`
		Success   int `json:"success,omitempty"`
		Failure   int `json:"failure,omitempty"`
	} `json:"metadata,omitempty"`
}

type OpenAPISearchTrack struct {
	Albums  []any `json:"albums"`
	Artists []any `json:"artists"`
	Tracks  []struct {
		Resource OpenAPITrackResource `json:"resource,omitempty"`
		ID       string               `json:"id"`
		Status   int                  `json:"status"`
		Message  string               `json:"message"`
	} `json:"tracks"`
	Videos []any `json:"videos"`
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

func (s *Service) GetTrackByISRC(isrc string) (*OpenAPIISRCTrackSearch, error) {
	var track OpenAPIISRCTrackSearch
	client := http.Client{}

	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", openAPIURL+"/tracks/byIsrc", nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+s.ClientAccessToken)
		req.Header.Set("accept", "application/vnd.tidal.v1+json")
		req.Header.Set("content-type", "application/vnd.tidal.v1+json")

		q := req.URL.Query()
		q.Add("isrc", strings.ToUpper(isrc))
		q.Add("countryCode", "US")
		q.Add("offset", "0")
		q.Add("limit", "1")
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Debug().Msg("Rate limited by Tidal API. Waiting 3 seconds before retrying.")
			time.Sleep(3 * time.Second)
			continue
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
			return nil, fmt.Errorf("failed to get track by ISRC: %s", resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(body, &track)
		if err != nil {
			return nil, err
		}

		if len(track.Data) == 0 {
			return nil, fmt.Errorf("track not found")
		}

		break
	}

	return &track, nil
}

func (s *Service) SearchTrack(query string, limit, offset int, country string, popularity string) (*OpenAPISearchTrack, error) {
	var track OpenAPISearchTrack
	client := http.Client{}

	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", openAPIURL+"/search", nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+s.ClientAccessToken)
		req.Header.Set("accept", "application/vnd.tidal.v1+json")
		req.Header.Set("content-type", "application/vnd.tidal.v1+json")

		q := req.URL.Query()
		q.Add("query", query)
		q.Add("type", "TRACKS")
		q.Add("offset", strconv.Itoa(offset))
		q.Add("limit", strconv.Itoa(limit))
		q.Add("countryCode", country)
		q.Add("popularity", popularity)
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Debug().Msg("Rate limited by Tidal API. Waiting 3 seconds before retrying.")
			time.Sleep(3 * time.Second)
			continue
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
			return nil, fmt.Errorf("failed to get track by ISRC: %s", resp.Status)
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(body, &track)
		if err != nil {
			return nil, err
		}

		break
	}

	return &track, nil
}
