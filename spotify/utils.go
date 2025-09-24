package spotify

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zibbp/spotify-playlist-sync/config"
	libSpotify "github.com/zmb3/spotify/v2"
)

type MissingTracks struct {
	Playlist libSpotify.SimplePlaylist `json:"playlist"`
	Tracks   []*libSpotify.FullTrack   `json:"tracks"`
}

// WriteMissingTracks writes missing tracks Spotify playlist tracks to disk
func WriteMissingTracks(filename string, missingTracks MissingTracks, config config.Config) error {
	if err := os.MkdirAll(config.DataPath+"/missing", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(missingTracks)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf(config.DataPath+"/missing/%s.json", filename), json, 0644)
}
