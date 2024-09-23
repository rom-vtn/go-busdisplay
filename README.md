# `go-busdisplay`
This is a really simple program I made to run on the max7219 array I hooked up to my RPi.

## How it works
- Program reads a `config.json` file given to it through args and acts as either a client or a server
- Client
  - Initializes the LED matrix array
  - Shows the system time
  - Periodically (every 30s) fetches and shows the current Spotify now playing
  - Periodically (every 30s) fetches and shows the next buses in the area it's been given
- Server
  - Has access to GTFS public transit data (will have to be fetched+unzipped first)
  - Has a Spotify access token for a registered app
  - When queried, queries the Spotify API and returns the next buses as well

## Example client config
```json
{
    "is_server": false,
    "tz_name": "Europe/Paris",
    "server_address": "https://yourserver.example.com",
    "spi_bus": 0,
    "spi_device": 0,
    "cascade_count": 8,
    "brightness": 0,
    "rotate_count": 3,
    "flip_horizontal": false,
    "flip_vertical": false,
    "latitude": 49,
    "longitude": 6
}
```

## Example server config
```json
{
    "is_server": true,
    "tz_name": "Europe/Paris",
    "host_port": 8888,
    "gtfs_directory_path": "/path/to/gtfs/dir/",
    "spotify_client_id": "CLIENT_ID",
    "spotify_client_secret": "SECRET",
    "spotify_refresh_token": "TOKEN"
}
```