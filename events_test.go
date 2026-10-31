package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRetainsMoreThan100RecentEventsAndPagesWithoutDuplicates(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	c.mu.Lock()
	for i := 0; i < 250; i++ {
		addEvent(&c.state, "info", fmt.Sprint(i))
	}
	c.mu.Unlock()
	status := c.Status()
	if len(status.Events) != 100 || status.EventCount != 250 {
		t.Fatalf("status must show first page but retain entire hour: %d/%d", len(status.Events), status.EventCount)
	}
	seen := map[uint64]bool{}
	before := uint64(0)
	pages := 0
	for {
		page := c.Events(before, 100)
		pages++
		for _, event := range page.Events {
			if seen[event.ID] {
				t.Fatal("duplicate event across pages")
			}
			seen[event.ID] = true
		}
		if !page.HasMore {
			break
		}
		if page.NextBefore == 0 {
			t.Fatal("missing next cursor")
		}
		before = page.NextBefore
	}
	if len(seen) != 250 || pages != 3 {
		t.Fatalf("history lost records: %d events in %d pages", len(seen), pages)
	}
}

func TestHistoryExpiresOldEventsOnReadsEvenWhenMonitoringStopped(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	c.mu.Lock()
	c.state.Events = []Event{
		{ID: 3, Time: time.Now().Add(-59 * time.Minute), Message: "recent"},
		{ID: 2, Time: time.Now().Add(-time.Hour - time.Millisecond), Message: "expired"},
		{ID: 1, Time: time.Now().Add(-2 * time.Hour), Message: "very old"},
	}
	c.mu.Unlock()
	page := c.Events(0, 100)
	if len(page.Events) != 1 || page.Events[0].Message != "recent" || page.Total != 1 {
		t.Fatalf("expired history returned: %+v", page)
	}
	if status := c.Status(); status.EventCount != 1 || len(status.Events) != 1 {
		t.Fatal("status returned expired history")
	}
	c.mu.Lock()
	remaining := len(c.state.Events)
	c.mu.Unlock()
	if remaining != 1 {
		t.Fatal("expired records were not cleaned from memory")
	}
}

func TestHistoryCursorUnaffectedByNewEvents(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	c.mu.Lock()
	for i := 0; i < 5; i++ {
		addEvent(&c.state, "info", fmt.Sprint(i))
	}
	c.mu.Unlock()
	first := c.Events(0, 2)
	c.mu.Lock()
	addEvent(&c.state, "info", "new")
	c.mu.Unlock()
	second := c.Events(first.NextBefore, 2)
	if len(second.Events) != 2 || second.Events[0].Message != "2" || second.Events[1].Message != "1" {
		t.Fatalf("new event shifted cursor: %+v", second)
	}
}

func TestHistoryEndpointBoundsAndValidation(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	c.mu.Lock()
	for i := 0; i < 120; i++ {
		addEvent(&c.state, "info", "test")
	}
	c.mu.Unlock()
	handler := NewServer(c, filepath.Join(t.TempDir(), "config.json"))
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=bad", "?before=-1", "?before=abc"} {
		r := httptest.NewRequest("GET", "http://localhost/api/events"+query, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Errorf("invalid pagination %s accepted: %d", query, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/events?limit=50", nil))
	var page EventPage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(page.Events) != 50 || page.Total != 120 || !page.HasMore || page.RetentionSeconds != 3600 {
		t.Fatalf("wrong event page: %+v", page)
	}
}
