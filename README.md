# discord-ical-srv

A Discord bot that synchronizes Discord events to a hosted iCal feed.

## Usage

Building: `go build`

Running:

```sh
export DISCORD_TOKEN="..."
./discord-ical-srv -l ":8080" # listen on port 8080
```

## API

```sh
# Documentation
$ curl -s localhost:8080

# Get all events from the given guild ID.
$ curl -s localhost:8080/guilds/$guildID/events
```
