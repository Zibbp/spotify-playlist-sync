package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

type MissingTracks struct {
	Playlist interface{} `json:"playlist"`
	Tracks   interface{} `json:"tracks"`
}

func WriteMissingTracks(filename string, playlist interface{}, tracks interface{}) error {
	if err := os.MkdirAll("/data/missing", 0755); err != nil {
		return err
	}
	data := MissingTracks{Playlist: playlist, Tracks: tracks}
	json, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("/data/missing/%s.json", filename), json, 0644)
}

func WriteTidalPlaylist(filename string, playlist interface{}) error {
	if err := os.MkdirAll("/data/tidal", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(playlist)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("/data/tidal/%s.json", filename), json, 0644)
}
