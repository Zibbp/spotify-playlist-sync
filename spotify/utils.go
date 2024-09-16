package spotify

import (
	"encoding/json"
	"fmt"
	"os"

	libSpotify "github.com/zmb3/spotify/v2"
)

type MissingTracks struct {
	Playlist libSpotify.SimplePlaylist `json:"playlist"`
	Tracks   []*libSpotify.FullTrack   `json:"tracks"`
}

// WriteMissingTracks writes missing tracks Spotify playlist tracks to disk
func WriteMissingTracks(filename string, missingTracks MissingTracks) error {
	if err := os.MkdirAll("/data/missing", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(missingTracks)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("/data/missing/%s.json", filename), json, 0644)
}
