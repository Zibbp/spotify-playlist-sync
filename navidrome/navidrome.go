package navidrome

import (
	"encoding/json"
	"fmt"
	"os"
)

type Playlist struct {
	SourceId      string  `json:"source_id"`
	DestinationId string  `json:"destination_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Tracks        []Track `json:"tracks"`
}

type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Album    string `json:"album"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration"`
	ISRC     string `json:"isrc"`
}

func WriteNavidromePlaylist(filename string, playlist interface{}) error {
	if err := os.MkdirAll("/data/navidrome", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(playlist)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("/data/navidrome/%s.json", filename), json, 0644)
}
