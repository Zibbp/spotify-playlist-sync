package deezer

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

type DeezerARL struct {
	Plan string
	Date string
	ARL  string
}

type DeezerUserData struct {
	Lossless bool
	Plan     string
	Country  string
}

func GetARLs() ([]DeezerARL, error) {

	// download string from url
	url := "https://rentry.org/firehawk52/raw"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error while downloading ARLs: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading ARLs: %v", err)
	}

	// Split the large string into lines
	lines := strings.Split(string(body), "\n")

	var arls []DeezerARL

	// Iterate over each line
	for _, line := range lines {
		// Check if the line starts with "->![United States of America]"
		if strings.HasPrefix(line, "->![United States of America]") {
			if !strings.Contains(line, "Deezer") {
				continue
			}

			// 0: Country
			// 1: Plan
			// 2: Date
			// 3: ARL
			parts := strings.Split(line, "|")
			if len(parts) < 2 {
				continue
			}

			arl := DeezerARL{
				Plan: strings.TrimSpace(strings.ReplaceAll(parts[1], "`", "")),
				Date: strings.TrimSpace(strings.ReplaceAll(parts[2], "`", "")),
				ARL:  strings.TrimSpace(strings.ReplaceAll(parts[3], "`", "")),
			}

			arls = append(arls, arl)

		}
	}

	log.Print("Parsed ARLs:")
	for _, arl := range arls {
		log.Printf("Plan: %s, Date: %s, ARL: %s", arl.Plan, arl.Date, arl.ARL)
	}

	return arls, nil
}

func TestARL(arl string) (*DeezerUserData, error) {

	sessionData := `{"api_token": "null", "api_version": "1.0", "input": "3", "method": "deezer.getUserData"}`

	arlCookie := http.Cookie{Name: "arl", Value: arl}

	req, err := http.NewRequest("POST", "http://www.deezer.com/ajax/gw-light.php", bytes.NewBufferString(sessionData))
	if err != nil {
		return nil, fmt.Errorf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&arlCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error while sending request: %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading response: %v", err)
	}

	var userData DeezerUserData

	country := gjson.Get(string(bodyBytes), "results.COUNTRY")
	lossless := gjson.Get(string(bodyBytes), "results.USER.OPTIONS.web_sound_quality.lossless")
	plan := gjson.Get(string(bodyBytes), "results.OFFER_NAME")
	if !country.Exists() || !lossless.Exists() || !plan.Exists() {
		return nil, fmt.Errorf("error while parsing response: %v", err)
	}

	userData.Country = country.String()
	userData.Lossless = lossless.Bool()
	userData.Plan = plan.String()

	return &userData, nil
}
