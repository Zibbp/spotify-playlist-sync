package tidal

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteTidalPlaylist writes Tidal playlist to disk
func WriteTidalPlaylist(filename string, playlist *Playlist) error {
	if err := os.MkdirAll("/data/tidal", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(playlist)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("/data/tidal/%s.json", filename), json, 0644)
}
