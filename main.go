package main

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Config struct {
	IsServer     bool   `json:"is_server"`
	TimezoneName string `json:"tz_name"`
	//server-side
	HostPort          uint16 `json:"host_port"`
	GtfsDirectoryPath string `json:"gtfs_directory_path"`
	//client-side
	ServerAddress  string  `json:"server_address"`
	SpiBus         uint    `json:"spi_bus"`
	SpiDevice      uint    `json:"spi_device"`
	CascadeCount   uint    `json:"cascade_count"`
	Brightness     uint    `json:"brightness"`
	RotateCount    uint    `json:"rotate_count"`
	FlipHorizontal bool    `json:"flip_horizontal"`
	FlipVertical   bool    `json:"flip_vertical"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
}

type Request struct {
	WantNowPlaying bool `json:"want_now_playing"`
	BusRequest     struct {
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		WantBuses bool    `json:"want_buses"`
	} `json:"bus_request"`
}

type Response struct {
	Success    bool             `json:"success"`
	Error      string           `json:"error"`
	NowPlaying NowPlayingResult `json:"now_playing"`
	NextBuses  []NextBusResult  `json:"next_buses"`
}

type NowPlayingResult struct {
	IsPlaying bool   `json:"is_playing"`
	Artist    string `json:"artist"`
	Title     string `json:"title"`
}
type NextBusResult struct {
	LineName    string    `json:"line_name"`
	Headsign    string    `json:"headsign"`
	PassingTime time.Time `json:"passing_time"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: ./display <path/to/config.json>")
	}

	configFilename := os.Args[1]
	configBytes, err := os.ReadFile(configFilename)
	if err != nil {
		log.Fatal("error while reading config file: ", err)
	}
	var config Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("error while parsing json config: ", err)
	}

	if config.IsServer {
		err = runServer(config)
		if err != nil {
			log.Fatal("error while running server: ", err)
		}
	} else {
		err = runClient(config)
		if err != nil {
			log.Fatal("error while running client: ", err)
		}
	}
}
