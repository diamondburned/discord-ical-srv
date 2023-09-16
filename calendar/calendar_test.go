package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const wantEmpty = `
BEGIN:VCALENDAR
PRODID:-//diamondburned//discord-ical-srv
VERSION:2.0
BEGIN:VEVENT
DTEND:00010101T000000Z
DTSTAMP:00010101T000000Z
DTSTART:00010101T000000Z
UID:
END:VEVENT
END:VCALENDAR
`

const wantEvents = `
BEGIN:VCALENDAR
PRODID:-//diamondburned//discord-ical-srv
VERSION:2.0
BEGIN:VEVENT
DESCRIPTION:This is a test event.
DTEND;TZID=America/Los_Angeles:19691231T170000
DTSTAMP;TZID=America/Los_Angeles:19691231T160000
DTSTART;TZID=America/Los_Angeles:19691231T160000
LOCATION:Los Angeles\, CA
SUMMARY:Test Event
UID:1
END:VEVENT
BEGIN:VEVENT
DESCRIPTION:This is a test event 2.
DTEND;TZID=America/Los_Angeles:19691231T171640
DTSTAMP;TZID=America/Los_Angeles:19691231T160000
DTSTART;TZID=America/Los_Angeles:19691231T161640
LOCATION:Anaheim\, CA
SUMMARY:Test Event 2
UID:2
END:VEVENT
END:VCALENDAR
`

func TestCalendar_WriteICS(t *testing.T) {
	tzLA, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		calendar Calendar
		want     string
	}{
		{
			name:     "empty",
			calendar: Calendar{},
			want:     wantEmpty,
		},
		{
			name: "events",
			calendar: Calendar{
				Events: []Event{
					{
						ID:          "1",
						CreatedAt:   time.Unix(0, 0).In(tzLA),
						Start:       time.Unix(0, 0).In(tzLA),
						End:         time.Unix(0, 0).In(tzLA).Add(time.Hour),
						Summary:     "Test Event",
						Description: "This is a test event.",
						Location:    "Los Angeles, CA",
					},
					{
						ID:          "2",
						CreatedAt:   time.Unix(0, 0).In(tzLA),
						Start:       time.Unix(1000, 0).In(tzLA),
						End:         time.Unix(1000, 0).In(tzLA).Add(time.Hour),
						Summary:     "Test Event 2",
						Description: "This is a test event 2.",
						Location:    "Anaheim, CA",
					},
				},
			},
			want: wantEvents,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			if err := tt.calendar.WriteICS(&b); err != nil {
				t.Fatalf("Calendar.WriteICS() error = %v", err)
			}

			t.Logf("\n%s", b.String())

			want := tt.want
			want = strings.TrimSpace(want)
			want = strings.ReplaceAll(want, "\n", "\r\n")

			got := b.String()
			got = strings.TrimSpace(got)

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Calendar.WriteICS() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
