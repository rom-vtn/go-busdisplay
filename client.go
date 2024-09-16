package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	gomax7219 "github.com/rom-vtn/gomax7219"
)

const DISPLAY_DELAY = 15 * time.Millisecond

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
	go getResponses(responseChan, 30*time.Second, config)
	responseChan <- Response{
		NowPlaying: NowPlayingResult{
			IsPlaying: true,
			Artist:    "Sample Artist",
			Title:     "Sample Song",
		},
		NextBuses: []NextBusResult{{
			Headsign:    "Headsign",
			LineName:    "Line",
			PassingTime: time.Now().Add(30 * time.Minute),
		}},
	}
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

	hpIcon := gomax7219.NewRawGridFromPattern(gomax7219.HeadphonesRefString)
	remainingWidth := config.CascadeCount*8 - hpIcon.GetWidth()

	nowPlayingRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, "NOW PLAYING")
	fittedNowPlayingRender := gomax7219.NewFitInsideGrid(nowPlayingRender, remainingWidth)

	artistTitleText := fmt.Sprintf("%s - %s", response.NowPlaying.Artist, response.NowPlaying.Title)
	artistTitleScrollingRender := gomax7219.NewScrollingGrid(
		gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, artistTitleText),
		remainingWidth)

	MIN_TIME := uint(150)
	sequenceDisplayTimes := []uint{MIN_TIME, max(MIN_TIME, artistTitleScrollingRender.GetFrameCount())}
	sequenceRenderers := []gomax7219.Renderer{fittedNowPlayingRender, artistTitleScrollingRender}
	sequence, err := gomax7219.NewSequenceGrid(sequenceRenderers, sequenceDisplayTimes)
	if err != nil {
		return err
	}

	concatRender := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{hpIcon, sequence})

	return ss.Draw(concatRender, DISPLAY_DELAY)
}

func displayClock(ss *gomax7219.SpiScreen, config Config) error {
	clockIcon := gomax7219.NewRawGridFromPattern(gomax7219.ClockRefString)
	timeString := time.Now().Format("15:04:05")
	timeRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, timeString)
	timeFitting := gomax7219.NewFitInsideGrid(timeRender, 8*config.CascadeCount-uint(len(clockIcon)))
	concat := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{clockIcon, timeFitting})
	err := ss.Draw(concat, DISPLAY_DELAY)
	if err != nil {
		return err
	}
	return nil
}

func getResponses(c chan Response, fetchDelay time.Duration, config Config) {
	for range time.Tick(fetchDelay) {
		response, err := sendServerRequest(config)
		if err != nil {
			log.Default().Printf("[INFO] could not get server response: %s\n", err.Error())
		}
		c <- response
	}
}

// represents a bus result entry *for display purposes* (contains the next 2 passing times)
type busResultEntry struct {
	lineName, headsign  string
	nextTime, afterNext time.Time
}

// tries adding the time to the passing times, returns true iff all times have just been set
func (bre *busResultEntry) addTime(passingTime time.Time) (hasAllSetTimes bool) {
	if bre.nextTime.IsZero() {
		bre.nextTime = passingTime
		return false
	}
	if bre.afterNext.IsZero() {
		bre.afterNext = passingTime
		return true
	}
	return false
}

func extractBusResultEntries(nextBuses []NextBusResult) []busResultEntry {
	now := time.Now()
	timelessToTimeful := make(map[busResultEntry]busResultEntry) //maps line+headsign to line+headsign+times
	fullEntryCount := 0                                          // full entry = has both times set
	const DISPLAY_COUNT int = 3
	for _, nextResult := range nextBuses {
		if fullEntryCount > DISPLAY_COUNT {
			break
		}
		if nextResult.PassingTime.Before(now) {
			continue
		}
		timelessEntry := busResultEntry{
			lineName: nextResult.LineName,
			headsign: nextResult.Headsign,
		}
		//read then rewrite
		timefulEntry, ok := timelessToTimeful[timelessEntry]
		if !ok {
			if len(timelessToTimeful) > DISPLAY_COUNT { //if all entries are there but not complete then stop adding
				continue
			}
			timefulEntry = timelessEntry //get headsign and line from timeless to init
		}
		isFull := timefulEntry.addTime(nextResult.PassingTime)
		if isFull {
			fullEntryCount++
		}
		timelessToTimeful[timelessEntry] = timefulEntry
	}
	var entriesToShow []busResultEntry
	for _, v := range timelessToTimeful {
		entriesToShow = append(entriesToShow, v)
	}
	sort.Slice(entriesToShow, func(i, j int) bool {
		return entriesToShow[i].nextTime.Before(entriesToShow[j].nextTime)
	})

	return entriesToShow
}

func displayBuses(ss *gomax7219.SpiScreen, response Response, config Config) error {
	nextBusResultEntries := extractBusResultEntries(response.NextBuses)

	for _, entry := range nextBusResultEntries {
		lineRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, entry.lineName)
		minutesLeftToNext := strconv.Itoa(int(time.Until(entry.nextTime).Minutes()))
		var minutesLeftToAfterNext string
		if !entry.afterNext.IsZero() {
			minutesLeftToAfterNext = strconv.Itoa(int(time.Until(entry.afterNext).Minutes()))
		} else {
			minutesLeftToAfterNext = "END"
		}
		minToNextRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, minutesLeftToNext)
		minToAfterNextRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, minutesLeftToAfterNext)
		timeRender, err := gomax7219.NewSequenceGrid([]gomax7219.Renderer{minToNextRender, minToAfterNextRender}, []uint{60, 60})
		if err != nil {
			return err
		}
		spaceLeftForHeadsign := 8*config.CascadeCount - lineRender.GetWidth() - timeRender.GetWidth()
		headsignRender := gomax7219.NewStringTextRender(gomax7219.ATARI_FONT, entry.headsign)
		scrollingHeadsign := gomax7219.NewScrollingGrid(headsignRender, spaceLeftForHeadsign)
		concatRender := gomax7219.NewConcatenateGrid([]gomax7219.Renderer{lineRender, scrollingHeadsign, timeRender})
		repeated := gomax7219.NewRepeatGrid(concatRender, 2)

		err = ss.Draw(repeated, DISPLAY_DELAY)
		if err != nil {
			return err
		}
	}

	return nil
}

func sendServerRequest(config Config) (Response, error) {
	req := Request{
		WantNowPlaying: true,
		BusRequest: struct {
			Lat       float64 "json:\"lat\""
			Lon       float64 "json:\"lon\""
			WantBuses bool    "json:\"want_buses\""
		}{
			Lat:       config.Latitude,
			Lon:       config.Longitude,
			WantBuses: true,
		},
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	httpResponse, err := http.DefaultClient.Post(config.ServerAddress, "application/json", bytes.NewReader(reqBytes))
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
