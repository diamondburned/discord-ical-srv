package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "embed"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jellydator/ttlcache/v3"
	"github.com/pkg/errors"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/sync/errgroup"
	"libdb.so/discord-ical-srv/calendar"
	"libdb.so/hserve"
	"tailscale.com/util/singleflight"
)

//go:embed README.md
var readme string

var (
	httpListenAddr     = "tcp://localhost:8080"
	calendarCacheTTL   = 5 * time.Minute
	calendarGCInterval = time.Hour
)

func init() {
	flag.StringVar(&httpListenAddr, "l", httpListenAddr, "HTTP listen address")
	flag.DurationVar(&calendarCacheTTL, "ttl", calendarCacheTTL, "Calendar cache TTL")
	flag.DurationVar(&calendarGCInterval, "gc", calendarGCInterval, "Calendar cache GC interval (0 to disable)")
}

func main() {
	flag.Parse()

	discordToken := os.Getenv("DISCORD_TOKEN")
	if discordToken == "" {
		log.Fatalln("$DISCORD_TOKEN is not set")
	}

	client := api.NewClient("Bot " + discordToken)
	calendarClient := newCalendarClient(client, calendarCacheTTL)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	errg, ctx := errgroup.WithContext(ctx)

	errg.Go(func() error {
		me, err := client.Me()
		if err != nil {
			return errors.Wrap(err, "failed to get user")
		}

		log.Println("Logged into Discord as", me.Username+"#"+me.Discriminator)
		return nil
	})

	errg.Go(func() error {
		handler := newCalendarServer(calendarClient)
		return hserve.ListenAndServe(ctx, httpListenAddr, handler)
	})

	if calendarGCInterval > 0 {
		errg.Go(func() error {
			ticker := time.NewTicker(calendarGCInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					calendarClient.cache.DeleteExpired()
				}
			}
		})
	}

	log.Println("Listening on", httpListenAddr)

	if err := errg.Wait(); err != nil {
		log.Fatalln(err)
	}
}

func newCalendarServer(client *calendarClient) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Compress(5))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, "<!DOCTYPE html>\n")
		io.WriteString(w, "<title>discord-ical-srv</title>\n")
		io.WriteString(w, `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/sakura.css@1.5.0/css/sakura-dark.css">`)
		io.WriteString(w, `<meta name="viewport" content="width=device-width, initial-scale=1">`)
		io.WriteString(w, markdownToHTML(readme))
	})

	r.Get("/guilds/{guildID}/events.ics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=events.ics")
		w.Header().Set("Cache-Control",
			fmt.Sprintf("public, revalidate, max-age=%d", int(client.ttl.Seconds())))

		idParam := chi.URLParam(r, "guildID")
		guildSnowflake, err := discord.ParseSnowflake(idParam)
		if err != nil {
			http.Error(w, "invalid guild ID", http.StatusBadRequest)
			return
		}
		guildID := discord.GuildID(guildSnowflake)

		events, err := client.Events(guildID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cal := calendar.Calendar{Events: events}
		if err := cal.WriteICS(w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	return r
}

type calendarClient struct {
	client *api.Client
	cache  *ttlcache.Cache[discord.GuildID, []calendar.Event]
	single *singleflight.Group[discord.GuildID, []calendar.Event]

	ttl time.Duration
}

func newCalendarClient(client *api.Client, ttl time.Duration) *calendarClient {
	return &calendarClient{
		client: client,
		single: new(singleflight.Group[discord.GuildID, []calendar.Event]),
		cache: ttlcache.New[discord.GuildID, []calendar.Event](
			ttlcache.WithTTL[discord.GuildID, []calendar.Event](5 * time.Minute),
		),
		ttl: ttl,
	}
}

func (c *calendarClient) Events(guildID discord.GuildID) ([]calendar.Event, error) {
	item := c.cache.Get(guildID)
	if item != nil {
		return item.Value(), nil
	}

	events, err, _ := c.single.Do(guildID, func() ([]calendar.Event, error) {
		discordEvents, err := c.client.ListScheduledEvents(guildID, false)
		if err != nil {
			return nil, err
		}

		events := convertDiscordEvents(discordEvents)
		c.cache.Set(guildID, events, c.ttl)

		return events, nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get events for guild %s", guildID)
	}

	return events, err
}

func convertDiscordEvents(discordEvents []discord.GuildScheduledEvent) []calendar.Event {
	events := make([]calendar.Event, len(discordEvents))
	for i, ev := range discordEvents {
		calEv := calendar.Event{
			ID:          ev.ID.String(),
			CreatedAt:   ev.ID.Time(),
			Start:       ev.StartTime.Time(),
			End:         ev.EndTime.Time(),
			Summary:     ev.Name,
			Description: markdownToHTML(ev.Description),
		}
		if ev.EntityMetadata != nil {
			calEv.Location = ev.EntityMetadata.Location
		}
		events[i] = calEv
	}
	return events
}

var mdRenderer = markdown.New(
	markdown.Breaks(true),
	markdown.Linkify(true),
	markdown.Typographer(true),
)

// markdownToHTML converts markdown to HTML.
func markdownToHTML(markdown string) string {
	return mdRenderer.RenderToString([]byte(markdown))
}
