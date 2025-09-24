package tidal

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/zibbp/spotify-playlist-sync/config"
)

// WriteTidalPlaylist writes Tidal playlist to disk
func WriteTidalPlaylist(filename string, playlist *Playlist, config config.Config) error {
	if err := os.MkdirAll(config.DataPath+"/tidal", 0755); err != nil {
		return err
	}
	json, err := json.Marshal(playlist)
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf(config.DataPath+"/tidal/%s.json", filename), json, 0644)
}

// parseISODuration converts an ISO 8601 duration (e.g., "P30M5S") to time.Duration
func ParseISODuration(isoDuration string) (time.Duration, error) {
	re := regexp.MustCompile(`P(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?`)
	matches := re.FindStringSubmatch(isoDuration)

	if matches == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration format")
	}

	var hours, minutes, seconds int
	var err error

	if matches[1] != "" {
		hours, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
	}
	if matches[2] != "" {
		minutes, err = strconv.Atoi(matches[2])
		if err != nil {
			return 0, err
		}
	}
	if matches[3] != "" {
		seconds, err = strconv.Atoi(matches[3])
		if err != nil {
			return 0, err
		}
	}

	totalSeconds := (hours * 3600) + (minutes * 60) + seconds
	return time.Duration(totalSeconds) * time.Second, nil
}
