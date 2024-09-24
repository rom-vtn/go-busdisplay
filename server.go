package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	nexttransit "github.com/rom-vtn/go-nexttransit"
)

var serverConfig Config

func runServer(config Config) error {
	serverConfig = config //set global var
	http.Handle("POST /", http.HandlerFunc(requestHandler))

	listenString := ":" + fmt.Sprint(config.HostPort)
	log.Default().Printf("Listening on %s", listenString)
	return http.ListenAndServe(listenString, nil)
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		sendError(w, err)
		return
	}
	var request Request
	err = json.Unmarshal(requestBody, &request)
	if err != nil {
		sendError(w, err)
		return
	}

	var busResults []NextBusResult
	if request.BusRequest.WantBuses {
		busResults, err = getBusResults(serverConfig, request)
		if err != nil {
			sendError(w, err)
			return
		}
	}
	var nowPlayingResult NowPlayingResult
	if request.WantNowPlaying {
		nowPlayingResult, err = getStatusAndUpdateToken(serverConfig)
		if err != nil {
			sendError(w, err)
			return
		}
	}

	response := Response{
		Success:    true,
		NextBuses:  busResults,
		NowPlaying: nowPlayingResult,
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		sendError(w, err)
		return
	}
	w.WriteHeader(200)
	_, err = w.Write(responseBytes)
	if err != nil {
		return
	}
}

func sendError(w http.ResponseWriter, err error) {
	w.WriteHeader(400)
	errResponse := Response{
		Success: false,
		Error:   err.Error(),
	}
	response, err := json.Marshal(errResponse)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(response)
}

func getBusResults(config Config, request Request) ([]NextBusResult, error) {
	tz, err := time.LoadLocation(config.TimezoneName)
	if err != nil {
		return nil, err
	}
	_, offSecs := time.Now().In(tz).Zone()
	sights, err := nexttransit.GetNextBuses(request.BusRequest.Lat, request.BusRequest.Lon, config.GtfsDirectoryPath, time.Now(), time.Duration(offSecs)*time.Second)
	if err != nil {
		return nil, err
	}
	var busResults []NextBusResult
	for _, sight := range sights {
		busResults = append(busResults, NextBusResult{
			PassingTime: sight.Timestamp,
			Headsign:    sight.Headsign,
			LineName:    sight.RouteName,
		})
	}
	return busResults, nil
}
