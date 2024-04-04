package convert

import (
	"fmt"
	"strings"

	"github.com/zibbp/spotify-playlist-convert/tidal"

	"github.com/rs/zerolog/log"
	spotifyPkg "github.com/zmb3/spotify/v2"
)

// clean up the track name removing everything after the first special character
func cleanName(trackName string) string {
	trackName = strings.TrimSpace(strings.Split(trackName, "-")[0])
	trackName = strings.TrimSpace(strings.Split(trackName, "(")[0])
	trackName = strings.TrimSpace(strings.Split(trackName, "[")[0])
	return trackName
}

func durationMatch(spotifyDuration int, tidalDuration int) bool {
	// allow for a 2 second difference
	log.Debug().Msgf("Spotify Duration: %d, Tidal Duration: %d", spotifyDuration, tidalDuration)
	return spotifyDuration >= tidalDuration-2 && spotifyDuration <= tidalDuration+2
}

func nameMatch(spotifyName string, tidalName string) bool {
	log.Debug().Msgf("Spotify Name: %s, Tidal Name: %s", spotifyName, tidalName)
	return strings.Contains(strings.ToLower(tidalName), strings.ToLower(spotifyName))
}

func artistMatch(spotifyArtists []string, tidalArtists []string) bool {
	log.Debug().Msgf("Spotify Artists: %v, Tidal Artists: %v", spotifyArtists, tidalArtists)
	for _, spotifyArtist := range spotifyArtists {
		found := false
		for _, tidalArtist := range tidalArtists {
			if strings.Contains(strings.ToLower(tidalArtist), strings.ToLower(spotifyArtist)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *Service) spotifyToTidalTrack(spotifyTrack *spotifyPkg.FullTrack) (*tidal.OpenAPITrackResource, error) {
	spotifyIsrc := spotifyTrack.ExternalIDs["isrc"]
	if spotifyIsrc != "" {
		// attempt to find the track using the ISRC
		tidalTrack, err := s.TidalService.GetTrackByISRC(spotifyIsrc)
		if err != nil {
			if err.Error() == "track not found" {
				log.Warn().Msgf("Track not found via ISRC: %s - %s (%s)", spotifyTrack.ID, spotifyTrack.Name, spotifyIsrc)
				// continue
			} else {
				return nil, err
			}
		}
		if tidalTrack != nil {
			return &tidalTrack.Data[0].Resource, nil
		}
	}

	// attempt to find track using name, artists, and duration
	spotifyArtists := make([]string, 0)
	for _, artist := range spotifyTrack.Artists {
		spotifyArtists = append(spotifyArtists, artist.Name)
	}

	spotifyName := cleanName(spotifyTrack.Name)
	// spotifyDuration := spotifyTrack.Duration

	spotifyAlbum := spotifyTrack.Album.Name

	// search #1
	query := fmt.Sprintf("%s %s", spotifyName, spotifyAlbum)

	log.Debug().Msgf("Searching for track: %s", query)

	tidalSearch, err := s.TidalService.SearchTrack(
		query,
		10,
		0,
		"US",
		"COUNTRY",
	)
	if err != nil {
		return nil, err
	}

	for _, tidalTrack := range tidalSearch.Tracks {
		// check if we have a match

		tidalArtists := make([]string, 0)
		for _, artist := range tidalTrack.Resource.Artists {
			tidalArtists = append(tidalArtists, artist.Name)
		}

		if nameMatch(spotifyName, tidalTrack.Resource.Title) && artistMatch(spotifyArtists, tidalArtists) && durationMatch(int((spotifyTrack.Duration/1000)), tidalTrack.Resource.Duration) {
			return &tidalTrack.Resource, nil
		}
	}

	// search #2
	query = fmt.Sprintf("%s %s", spotifyName, spotifyArtists[0])

	log.Debug().Msgf("Searching for track: %s", query)

	tidalSearch, err = s.TidalService.SearchTrack(
		query,
		10,
		0,
		"US",
		"COUNTRY",
	)
	if err != nil {
		return nil, err
	}

	for _, tidalTrack := range tidalSearch.Tracks {
		// check if we have a match

		tidalArtists := make([]string, 0)
		for _, artist := range tidalTrack.Resource.Artists {
			tidalArtists = append(tidalArtists, artist.Name)
		}

		if nameMatch(spotifyName, tidalTrack.Resource.Title) && artistMatch(spotifyArtists, tidalArtists) && durationMatch(int((spotifyTrack.Duration/1000)), tidalTrack.Resource.Duration) {
			return &tidalTrack.Resource, nil
		}
	}

	return nil, nil
}
