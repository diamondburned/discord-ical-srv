package calendar

import (
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-ical"
)

// Calendar is a calendar.
type Calendar struct {
	Events []Event
}

// Event is a calendar event.
type Event struct {
	// ID is the ID of the event.
	ID string
	// CreatedAt is the time the event was created.
	CreatedAt time.Time
	// Start is the start time of the event.
	Start time.Time
	// End is the end time of the event.
	End time.Time
	// Summary is the title of the event. It is optional.
	Summary string
	// Description is the description of the event. It is assumed to be in
	// HTML format. It is optional.
	Description string
	// Location is the location of the event. It is optional.
	Location string
}

// WriteICS writes the calendar as an ICS file into the given writer and returns
// any error encountered.
func (c Calendar) WriteICS(w io.Writer) error {
	ics := ical.NewCalendar()
	ics.Props.SetText(ical.PropVersion, "2.0")
	ics.Props.SetText(ical.PropProductID, "-//diamondburned//discord-ical-srv")

	for i, event := range c.Events {
		if event.ID == "" {
			return fmt.Errorf("event %d has no ID", i)
		}
		if event.CreatedAt.IsZero() {
			return fmt.Errorf("event %d has no creation time", i)
		}
		if event.Start.IsZero() {
			return fmt.Errorf("event %d has no start time", i)
		}
		if event.End.IsZero() {
			return fmt.Errorf("event %d has no end time", i)
		}

		ev := ical.NewEvent()
		ev.Props.SetText(ical.PropUID, event.ID)
		ev.Props.SetDateTime(ical.PropDateTimeStamp, event.CreatedAt)
		ev.Props.SetDateTime(ical.PropDateTimeStart, event.Start)
		ev.Props.SetDateTime(ical.PropDateTimeEnd, event.End)

		if event.Summary != "" {
			ev.Props.SetText(ical.PropSummary, event.Summary)
		}
		if event.Description != "" {
			ev.Props.SetText(ical.PropDescription, event.Description)
		}
		if event.Location != "" {
			ev.Props.SetText(ical.PropLocation, event.Location)
		}

		ics.Children = append(ics.Children, ev.Component)
	}

	if len(ics.Children) == 0 {
		// Insert a dummy event that happened way in the past, so that the
		// calendar isn't empty.
		// See https://stackoverflow.com/a/58310100/5041327.
		early := (time.Time{}).In(time.UTC)

		ev := ical.NewEvent()
		ev.Props.SetText(ical.PropUID, "")
		ev.Props.SetDateTime(ical.PropDateTimeStamp, early)
		ev.Props.SetDateTime(ical.PropDateTimeStart, early)
		ev.Props.SetDateTime(ical.PropDateTimeEnd, early)

		ics.Children = append(ics.Children, ev.Component)
	}

	enc := ical.NewEncoder(w)
	return enc.Encode(ics)
}
