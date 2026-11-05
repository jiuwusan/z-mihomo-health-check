package main

import "time"

const eventRetention = time.Hour
const eventPageSize = 100

type EventPage struct {
	Events           []Event `json:"events"`
	Total            int     `json:"total"`
	HasMore          bool    `json:"has_more"`
	NextBefore       uint64  `json:"next_before"`
	RetentionSeconds int     `json:"retention_seconds"`
	HistoryID        string  `json:"history_id"`
}

// Clean on writes and reads, including while monitoring is stopped.
func pruneEvents(s *Status, now time.Time) {
	cutoff := now.Add(-eventRetention)
	kept := s.Events[:0]
	for _, event := range s.Events {
		if !event.Time.Before(cutoff) && !event.Time.After(now) {
			kept = append(kept, event)
		}
	}
	clear(s.Events[len(kept):])
	s.Events = kept
	s.EventCount = len(kept)
}

func (c *Controller) Events(before uint64, limit int) EventPage {
	return c.filteredEvents(before, limit, false)
}

func (c *Controller) filteredEvents(before uint64, limit int, failuresOnly bool) EventPage {
	c.mu.Lock()
	defer c.mu.Unlock()
	pruneEvents(&c.state, time.Now())
	return historyPage(&c.state, before, limit, failuresOnly)
}

func historyPage(s *Status, before uint64, limit int, failuresOnly bool) EventPage {
	if limit < 1 || limit > eventPageSize {
		limit = eventPageSize
	}
	page := EventPage{Events: []Event{}, RetentionSeconds: int(eventRetention / time.Second), HistoryID: s.HistoryID}
	if !failuresOnly {
		page.Total = len(s.Events)
	}
	for _, event := range s.Events {
		if failuresOnly {
			if event.Level != "warning" && event.Level != "error" {
				continue
			}
			page.Total++
		}
		if before != 0 && event.ID >= before {
			continue
		}
		if len(page.Events) == limit {
			page.HasMore = true
			if !failuresOnly {
				break
			}
			continue
		}
		page.Events = append(page.Events, event)
		page.NextBefore = event.ID
	}
	return page
}
