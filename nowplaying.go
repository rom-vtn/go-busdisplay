package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

//shamelessly copy-pasted from my https://github.com/rom-vtn/nowplaying

type SpotifyTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type SpotifyNowPlayingRepsonse struct {
	RepeatState          string      `json:"repeat_state"`
	ShuffleState         bool        `json:"shuffle_state"`
	ProgressMs           int         `json:"progress_ms"`
	IsPlaying            bool        `json:"is_playing"`
	CurrentlyPlayingType string      `json:"currently_playing_type"`
	Item                 TrackObject `json:"item"`
}

type TrackObject struct {
	Album      Album         `json:"album"`   //songs only
	Artists    []Artist      `json:"artists"` //songs only
	DurationMs int           `json:"duration_ms"`
	Name       string        `json:"name"`
	Id         string        `json:"id"`
	Images     []ImageObject `json:"images"`
	Show       Show          `json:"show"` //episodes only
}

type Show struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
}

type Album struct {
	AlbumType   string        `json:"album_type"`
	TotalTracks int           `json:"total_tracks"`
	Id          string        `json:"id"`
	Images      []ImageObject `json:"images"`
	Name        string        `json:"name"`
}

type Artist struct {
	Id     string        `json:"id"`
	Images []ImageObject `json:"images"`
	Name   string        `json:"name"`
}

type ImageObject struct {
	Url    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

func refreshToken(config Config) (string, error) {
	clientId := config.SpotifyClientId
	clientSecret := config.SpotifyClientSecret
	refreshToken := config.SpotifyRefreshToken

	body := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s", refreshToken)
	httpRequest, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientId+":"+clientSecret)))

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return "", err
	}

	responseBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return "", err
	}

	var response SpotifyTokenResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return "", err
	}

	if response.AccessToken == "" {
		return "", fmt.Errorf("no access token returned")
	}

	return response.AccessToken, nil
}

func getCurrentlyPlaying(token string) (SpotifyNowPlayingRepsonse, error) {
	httpRequest, err := http.NewRequest("GET", "https://api.spotify.com/v1/me/player?additional_types=episode", nil)
	if err != nil {
		return SpotifyNowPlayingRepsonse{}, err
	}
	httpRequest.Header.Add("Authorization", "Bearer "+token)

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return SpotifyNowPlayingRepsonse{}, err
	}

	// handle HTTP 204 accordingly
	// NOTE: this will set the field `IsPlaying` to `false`
	if httpResponse.StatusCode == 204 {
		return SpotifyNowPlayingRepsonse{}, nil
	}

	if httpResponse.StatusCode != 200 {
		return SpotifyNowPlayingRepsonse{}, fmt.Errorf("didnt get HTTP 200 from API, got %d instead", httpResponse.StatusCode)
	}

	responseBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return SpotifyNowPlayingRepsonse{}, err
	}

	var response SpotifyNowPlayingRepsonse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return SpotifyNowPlayingRepsonse{}, err
	}

	return response, nil
}

var currentToken string
var currentTokenDate time.Time

func getStatusAndUpdateToken(config Config) (NowPlayingResult, error) {
	//ensure we have a token
	if currentTokenDate.Add(time.Hour).Before(time.Now()) {
		token, err := refreshToken(config)
		if err != nil {
			return NowPlayingResult{}, err
		}
		currentTokenDate = time.Now()
		currentToken = token
	}

	//get the status
	snpr, err := getCurrentlyPlaying(currentToken)
	if err != nil {
		return NowPlayingResult{}, err
	}

	if !snpr.IsPlaying { //if nothing playing, don't crash
		return NowPlayingResult{}, nil
	}

	npr := NowPlayingResult{
		IsPlaying:   snpr.IsPlaying,
		ContentType: snpr.CurrentlyPlayingType,
		Artist:      snpr.Item.Artists[0].Name,
		Title:       snpr.Item.Name,
	}
	return npr, nil
}
