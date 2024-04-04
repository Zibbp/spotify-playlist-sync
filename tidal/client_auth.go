package tidal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) clientAuth(clientId string, clientSecret string) (string, error) {

	client := &http.Client{}

	params := url.Values{}
	params.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/token", authURL), strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(clientId, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to authenticate client: %s", resp.Status)
	}

	// parse json response without struct
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("failed to parse access token")
	}

	return accessToken, nil
}
