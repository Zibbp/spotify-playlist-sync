package tidal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

const countryCode = "US"

type CreatedPlaylist struct {
	Trn            string      `json:"trn"`
	ItemType       string      `json:"itemType"`
	AddedAt        string      `json:"addedAt"`
	LastModifiedAt string      `json:"lastModifiedAt"`
	Name           string      `json:"name"`
	Parent         interface{} `json:"parent"`
	Data           Playlist    `json:"data"`
}

type Playlist struct {
	UUID            string           `json:"uuid"`
	Title           string           `json:"title"`
	NumberOfTracks  int64            `json:"numberOfTracks"`
	NumberOfVideos  int64            `json:"numberOfVideos"`
	Creator         Creator          `json:"creator"`
	Description     string           `json:"description"`
	Duration        int64            `json:"duration"`
	LastUpdated     string           `json:"lastUpdated"`
	Created         string           `json:"created"`
	Type            string           `json:"type"`
	PublicPlaylist  bool             `json:"publicPlaylist"`
	URL             string           `json:"url"`
	Image           string           `json:"image"`
	Popularity      int64            `json:"popularity"`
	SquareImage     string           `json:"squareImage"`
	PromotedArtists []PromotedArtist `json:"promotedArtists"`
	LastItemAddedAt string           `json:"lastItemAddedAt"`
	Tracks          []Track          `json:"tracks"`
}

type Creator struct {
	ID int64 `json:"id"`
}

type PromotedArtist struct {
	ID      int64       `json:"id"`
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Picture interface{} `json:"picture"`
}

type TidalPlaylistTracks struct {
	Limit              int64   `json:"limit"`
	Offset             int64   `json:"offset"`
	TotalNumberOfItems int64   `json:"totalNumberOfItems"`
	Items              []Track `json:"items"`
}

type Track struct {
	ID                   int64       `json:"id"`
	Title                string      `json:"title"`
	Duration             int64       `json:"duration"`
	ReplayGain           float64     `json:"replayGain"`
	Peak                 float64     `json:"peak"`
	AllowStreaming       bool        `json:"allowStreaming"`
	StreamReady          bool        `json:"streamReady"`
	StreamStartDate      *string     `json:"streamStartDate"`
	PremiumStreamingOnly bool        `json:"premiumStreamingOnly"`
	TrackNumber          int64       `json:"trackNumber"`
	VolumeNumber         int64       `json:"volumeNumber"`
	Version              *string     `json:"version"`
	Popularity           int64       `json:"popularity"`
	Copyright            string      `json:"copyright"`
	Description          interface{} `json:"description"`
	URL                  string      `json:"url"`
	Isrc                 string      `json:"isrc"`
	Editable             bool        `json:"editable"`
	Explicit             bool        `json:"explicit"`
	AudioQuality         string      `json:"audioQuality"`
	AudioModes           []string    `json:"audioModes"`
	Artist               Artist      `json:"artist"`
	Artists              []Artist    `json:"artists"`
	Album                Album       `json:"album"`
	Mixes                Mixes       `json:"mixes"`
	DateAdded            string      `json:"dateAdded"`
	Index                int64       `json:"index"`
	ItemUUID             string      `json:"itemUuid"`
	NumberOfTracks       int64       `json:"numberOfTracks"`
}

type Album struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Cover        string  `json:"cover"`
	VibrantColor string  `json:"vibrantColor"`
	VideoCover   *string `json:"videoCover"`
	ReleaseDate  string  `json:"releaseDate"`
}

type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Picture string `json:"picture"`
}

type Mixes struct {
	MasterTrackMix *string `json:"MASTER_TRACK_MIX,omitempty"`
	TrackMix       string  `json:"TRACK_MIX"`
}

type TrackSearch struct {
	Artists   SearchTracksPagination `json:"artists"`
	Albums    SearchTracksPagination `json:"albums"`
	Playlists SearchTracksPagination `json:"playlists"`
	Tracks    SearchTracksPagination `json:"tracks"`
	Videos    SearchTracksPagination `json:"videos"`
	TopHit    TopHit                 `json:"topHit"`
}

type SearchTracksPagination struct {
	Limit              int64   `json:"limit"`
	Offset             int64   `json:"offset"`
	TotalNumberOfItems int64   `json:"totalNumberOfItems"`
	Items              []Track `json:"items"`
}

type UserPlaylists struct {
	Limit              int64      `json:"limit"`
	Offset             int64      `json:"offset"`
	TotalNumberOfItems int64      `json:"totalNumberOfItems"`
	Items              []Playlist `json:"items"`
}

type TopHit struct {
	Value Track  `json:"value"`
	Type  string `json:"type"`
}

func (s *Service) standardHttpGetRequest(reqUrl string) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}

	// Set Headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken))

	// Set Query Params
	q := url.Values{}
	q.Add("countryCode", countryCode)
	q.Add("limit", "10000")

	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", string(body))
	}

	return body, nil
}

func (s *Service) GetUserPlaylists() (UserPlaylists, error) {
	playlists, err := s.standardHttpGetRequest(fmt.Sprintf("%s/users/%s/playlists", apiURL, s.UserID))
	if err != nil {
		return UserPlaylists{}, err
	}

	var tidalUserPlaylists UserPlaylists
	err = json.Unmarshal(playlists, &tidalUserPlaylists)
	if err != nil {
		return UserPlaylists{}, err
	}

	return tidalUserPlaylists, nil
}

func (s *Service) GetPlaylist(playlistID string) (*Playlist, error) {
	playlist, err := s.standardHttpGetRequest(fmt.Sprintf("%s/playlists/%s", apiURL, playlistID))
	if err != nil {
		return nil, err
	}

	var tidalPlaylist Playlist
	err = json.Unmarshal(playlist, &tidalPlaylist)
	if err != nil {
		return nil, err
	}

	return &tidalPlaylist, nil
}

func (s *Service) CreatePlaylist(name, description string) (*Playlist, error) {
	client := &http.Client{}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/my-collection/playlists/folders/create-playlist", apiURL2), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)

	params := url.Values{}
	params.Set("folderId", "root")
	params.Set("name", name)
	params.Set("description", description)

	req.URL.RawQuery = params.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create playlist: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var createdPlaylist CreatedPlaylist
	err = json.Unmarshal(body, &createdPlaylist)
	if err != nil {
		return nil, err
	}

	return &createdPlaylist.Data, nil
}

func (s *Service) UpdatePlaylist(playlistID, name, description string) error {
	// updated name and description sent in body no params
	client := &http.Client{}

	data := url.Values{}
	data.Set("title", name)
	data.Set("description", description)

	encodedData := data.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/playlists/%s", apiURL, playlistID), strings.NewReader(encodedData))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to update playlist: %s", resp.Status)
	}

	return nil
}

func (s *Service) getPlaylistEtag(id string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/playlists/%s", apiURL, id), nil)
	if err != nil {
		return "", err
	}

	// Set Headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken))

	// Set Query Params
	q := url.Values{}
	q.Add("countryCode", countryCode)

	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", string(body))
	}

	playlistEtag := resp.Header.Get("ETag")

	return playlistEtag, nil
}

func (s *Service) AddTrackToPlaylist(playlistId string, trackId string) error {
	playlistEtag, err := s.getPlaylistEtag(playlistId)
	if err != nil {
		return err
	}

	client := &http.Client{}

	data := url.Values{}
	data.Set("trackIds", fmt.Sprintf("%v", trackId))
	data.Set("onArtifactNotFound", "FAIL")
	data.Set("onDupes", "FAIL")

	encodedData := data.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/playlists/%s/items", apiURL, playlistId), strings.NewReader(encodedData))
	if err != nil {
		return err
	}

	// Set Headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken))
	req.Header.Set("If-None-Match", playlistEtag)

	// Set Query Params
	q := url.Values{}
	q.Add("countryCode", countryCode)

	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			log.Debug().Msgf("Track %v already exists in playlist %s", trackId, playlistId)
		} else {
			return fmt.Errorf("failed to add track to playlist: %s", string(body))
		}

		return err
	}

	return nil
}

func (s *Service) GetPlaylistTracks(id string) (*TidalPlaylistTracks, error) {
	body, err := s.standardHttpGetRequest(fmt.Sprintf("%s/playlists/%s/tracks", apiURL, id))
	if err != nil {
		return nil, err
	}

	var tidalPlaylistTracks TidalPlaylistTracks
	err = json.Unmarshal(body, &tidalPlaylistTracks)
	if err != nil {
		return nil, err
	}

	return &tidalPlaylistTracks, nil
}
