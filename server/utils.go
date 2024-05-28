package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type TimeResponse struct {
	UnixTime int64 `json:"unixtime"`
}

func GetUnixTime() (int64, error) {
	url := "https://worldtimeapi.org/api/timezone/Asia/Karachi"
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch data from API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	var timeResponse TimeResponse
	err = json.Unmarshal(body, &timeResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return timeResponse.UnixTime, nil
}
