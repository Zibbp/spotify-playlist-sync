// The Tidal API does not supprot user authentication flows yet.
// Use the not-so-public device authentication flow to authenticate with Tidal for user requests
package tidal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	authURL = "https://auth.tidal.com/v1/oauth2"
	// API Keys - https://github.com/yaronzz/Tidal-Media-Downloader/blob/bb5be5e5fba3a648cdda9c8b46c707682fb5472c/TIDALDL-PY/tidal_dl/apiKey.py
	clientId     = "7m7Ap0JC9j1cOM3n"
	clientSecret = "vRAdA108tlvkJpTsGZS8rGZ7xTlbJ0qaZ2K9saEzsgY="
)

type DeviceCode struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

type LoginResponse struct {
	AuthLogin AuthLogin
	AuthError AuthError
}

type AuthLogin struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}

type Refresh struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	User        User   `json:"user"`
}

type User struct {
	UserID       int64       `json:"userId"`
	Email        interface{} `json:"email"`
	CountryCode  string      `json:"countryCode"`
	FullName     interface{} `json:"fullName"`
	FirstName    interface{} `json:"firstName"`
	LastName     interface{} `json:"lastName"`
	Nickname     interface{} `json:"nickname"`
	Username     string      `json:"username"`
	Address      interface{} `json:"address"`
	City         interface{} `json:"city"`
	Postalcode   interface{} `json:"postalcode"`
	UsState      interface{} `json:"usState"`
	PhoneNumber  interface{} `json:"phoneNumber"`
	Birthday     interface{} `json:"birthday"`
	Gender       interface{} `json:"gender"`
	ImageID      interface{} `json:"imageId"`
	ChannelID    int64       `json:"channelId"`
	ParentID     int64       `json:"parentId"`
	AcceptedEULA bool        `json:"acceptedEULA"`
	Created      int64       `json:"created"`
	Updated      int64       `json:"updated"`
	FacebookUid  int64       `json:"facebookUid"`
	AppleUid     interface{} `json:"appleUid"`
	GoogleUid    interface{} `json:"googleUid"`
	NewUser      bool        `json:"newUser"`
}

type AuthError struct {
	Status           int64  `json:"status"`
	Error            string `json:"error"`
	SubStatus        int64  `json:"sub_status"`
	ErrorDescription string `json:"error_description"`
}

type Session struct {
	SessionID   string `json:"sessionId"`
	UserID      int64  `json:"userId"`
	CountryCode string `json:"countryCode"`
	ChannelID   int64  `json:"channelId"`
	PartnerID   int64  `json:"partnerId"`
	Client      Client `json:"client"`
}

type Client struct {
	ID                       int64       `json:"id"`
	Name                     string      `json:"name"`
	AuthorizedForOffline     bool        `json:"authorizedForOffline"`
	AuthorizedForOfflineDate interface{} `json:"authorizedForOfflineDate"`
}

func (s *Service) getDeviceCode() (*DeviceCode, error) {
	var deviceCode DeviceCode

	client := &http.Client{}

	data := url.Values{}
	data.Set("client_id", clientId)
	data.Set("scope", "r_usr+w_usr+w_sub")

	encodedData := data.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/device_authorization", authURL), strings.NewReader(encodedData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get device code: %s", resp.Status)
	}

	err = json.Unmarshal(body, &deviceCode)
	if err != nil {
		return nil, err
	}

	return &deviceCode, nil
}

func (s *Service) tokenLogin(deviceCode DeviceCode) (*LoginResponse, error) {
	var loginResponse LoginResponse

	client := &http.Client{}
	// Set body
	data := url.Values{}
	data.Set("client_id", clientId)
	data.Set("device_code", deviceCode.DeviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("scope", "r_usr+w_usr+w_sub")

	encodedData := data.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/token", authURL), strings.NewReader(encodedData))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientId, clientSecret)

	// Set Headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		var authError AuthError
		err = json.Unmarshal(body, &authError)
		if err != nil {
			return nil, err
		}
		loginResponse.AuthError = authError
		return &loginResponse, err
	}

	var authLogin AuthLogin
	err = json.Unmarshal(body, &authLogin)
	if err != nil {
		return nil, err
	}
	loginResponse.AuthLogin = authLogin

	return &loginResponse, nil
}

func (s *Service) checkSession(accessToken string) (Session, error) {
	var session Session

	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/sessions", apiURL), nil)
	if err != nil {
		return session, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return session, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return session, err
	}

	if resp.StatusCode != 200 {
		return session, fmt.Errorf("failed to check session: %s", resp.Status)
	}

	err = json.Unmarshal(body, &session)
	if err != nil {
		return session, err
	}

	return session, nil
}

func (s *Service) refreshSession(refreshToken string) (*Refresh, error) {
	var refresh Refresh

	client := &http.Client{}

	data := url.Values{}
	data.Set("client_id", clientId)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")
	data.Set("scope", "r_usr+w_usr+w_sub")

	encodedData := data.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/token", authURL), strings.NewReader(encodedData))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientId, clientSecret)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to refresh session: %s", resp.Status)
	}

	err = json.Unmarshal(body, &refresh)
	if err != nil {
		return nil, err
	}

	return &refresh, nil
}
