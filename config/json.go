package config

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type JsonConfig struct {
	Spotify SpotifyConfig `json:"spotify"`
	Tidal   TidalConfig   `json:"tidal"`
}

type SpotifyConfig struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
}

type TidalConfig struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type JsonConfigService struct {
	mu         sync.Mutex
	JsonConfig JsonConfig
	Path       string
}

func NewJsonConfigService(path string) *JsonConfigService {
	return &JsonConfigService{
		Path: path,
	}
}

func (s *JsonConfigService) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.Path)
	if err != nil {
		// Create a new config file if it doesn't exist
		if os.IsNotExist(err) {
			s.JsonConfig = JsonConfig{}
			return s.Save()
		}
		return err
	}
	defer file.Close()

	// read json
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var config JsonConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	s.JsonConfig = config

	return nil
}

func (s *JsonConfigService) Save() error {
	data, err := json.MarshalIndent(s.JsonConfig, "", "	")
	if err != nil {
		return err
	}

	if err := os.WriteFile(s.Path, data, 0644); err != nil {
		return err
	}

	return nil
}

func (s *JsonConfigService) Get() JsonConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.JsonConfig
}

func (s *JsonConfigService) Update(config JsonConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.JsonConfig = config

	if err := s.Save(); err != nil {
		return err
	}

	return nil
}
