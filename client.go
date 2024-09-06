package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	gomax7219 "github.com/rom-vtn/gomax7219"
)

func runClient(config Config) error {
	ss, err := gomax7219.NewDeviceAndOpen(
		config.SpiBus,
		config.SpiDevice,
		config.CascadeCount,
		config.Brightness,
		config.RotateCount,
		config.FlipHorizontal,
		config.FlipVertical)
	if err != nil {
		return err
	}
	defer ss.Close()

	//get responses every 30 secs
	responseChan := make(chan Response, 5) //add a bit of buffer
	go getResponses(responseChan, 30*time.Second)

	for response := range responseChan {

		err = displayBuses(ss, response, config)
		if err != nil {
			return err
		}

		err = displayNowPlaying(ss, response, config)
		if err != nil {
			return err
		}

		for len(responseChan) == 0 {
			err := displayClock(ss, config)
			if err != nil {
				return err
			}
		}

	}

	return errors.New("uh oh, we weren't really supposed to leave the loop")
}

func displayNowPlaying(ss *gomax7219.SpiScreen, response Response, config Config) error {
	//has to actually play something to render
	if !response.NowPlaying.IsPlaying {
		return nil
	}

	hpIcon := gomax7219.NewRawGridFromPattern(gomax7219.TrainRefString) //TODO replace with headphone icon
	remainingWidth := config.CascadeCount*8 - hpIcon.GetWidth()

	nowPlayingText := fmt.Sprintf("Now Playing: %s - %s", response.NowPlaying.Artist, response.NowPlaying.Title)
	scrollingRender := gomax7219.NewScrollingGrid(
		gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, nowPlayingText),
		remainingWidth)
	concatRender := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{hpIcon, scrollingRender})

	return ss.Draw(concatRender, 100*time.Millisecond)
}

func displayClock(ss *gomax7219.SpiScreen, config Config) error {
	clockIcon := gomax7219.NewRawGridFromPattern(gomax7219.ClockRefString)
	timeString := time.Now().Format("15:04:05")
	timeRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, timeString)
	timeFitting := gomax7219.NewFitInsideGrid(timeRender, 8*config.CascadeCount-uint(len(clockIcon)))
	concat := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{clockIcon, timeFitting})
	err := ss.Draw(concat, 100*time.Millisecond)
	if err != nil {
		return err
	}
	return nil
}

func getResponses(c chan Response, fetchDelay time.Duration) {
	for range time.Tick(fetchDelay) {
		response, err := sendServerRequest()
		if err != nil {
			log.Default().Printf("[INFO] could not get server response: %s\n", err.Error())
		}
		c <- response
	}
}

func displayBuses(ss *gomax7219.SpiScreen, response Response, config Config) error {
	now := time.Now()

	tramIcon := gomax7219.NewRawGridFromPattern(gomax7219.TramRefString)

	var currentNextPassingTimes []NextBusResult
	for _, nextResult := range response.NextBuses {
		if len(currentNextPassingTimes) > 3 {
			break
		}
		if nextResult.PassingTime.Before(now) {
			continue
		}
		currentNextPassingTimes = append(currentNextPassingTimes, nextResult)
	}

	for _, npt := range currentNextPassingTimes {
		minutesLeft := time.Until(npt.PassingTime).Minutes()
		timeRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, strconv.Itoa(int(minutesLeft)))
		spaceLeftForHeadsign := 8*int(config.CascadeCount) - len(tramIcon) - len(timeRender)
		headsignRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, npt.Headsign)
		scrollingHeadsign := gomax7219.NewScrollingGrid(headsignRender, uint(spaceLeftForHeadsign))
		concatRender := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{tramIcon, scrollingHeadsign, timeRender})
		repeated := gomax7219.NewRepeatGrid(concatRender, 2)

		err := ss.Draw(repeated, 100*time.Millisecond)
		if err != nil {
			return err
		}
	}

	return nil
}

func sendServerRequest() (Response, error) {
	req := Request{
		WantNowPlaying: true,
		BusRequest: struct {
			Lat       float64 "json:\"lat\""
			Lon       float64 "json:\"lon\""
			WantBuses bool    "json:\"want_buses\""
		}{
			Lat:       serverConfig.Latitude,
			Lon:       serverConfig.Longitude,
			WantBuses: true,
		},
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	httpResponse, err := http.DefaultClient.Post(serverConfig.ServerAddress, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return Response{}, err
	}

	respBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return Response{}, err
	}

	var reponse Response
	err = json.Unmarshal(respBytes, &reponse)
	if err != nil {
		return Response{}, err
	}
	return reponse, nil
}
