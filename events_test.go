package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestFailureFilterAppliesBeforePagingAndStatusKeepsMetrics(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	c.mu.Lock()
	addEvent(&c.state, "error", "old API failure")
	addEvent(&c.state, "warning", "old probe failure")
	for i := 0; i < 150; i++ {
		addEvent(&c.state, "success", "healthy")
	}
	c.state.Failures = 7
	c.mu.Unlock()
	h := NewServer(c, filepath.Join(t.TempDir(), "config.json"))
	status := httptest.NewRecorder()
	h.ServeHTTP(status, httptest.NewRequest("GET", "/api/status?failures_only=true", nil))
	var s Status
	if err := json.Unmarshal(status.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if status.Code != 200 || s.EventCount != 2 || len(s.Events) != 2 || s.Failures != 7 {
		t.Fatalf("wrong filtered status: %+v", s)
	}
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest("GET", "/api/events?failures_only=true&limit=1", nil))
	var p EventPage
	if err := json.Unmarshal(first.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if first.Code != 200 || p.Total != 2 || len(p.Events) != 1 || !p.HasMore || p.Events[0].Level != "warning" {
		t.Fatal(p)
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest("GET", fmt.Sprintf("/api/events?failures_only=true&limit=1&before=%d", p.NextBefore), nil))
	if err := json.Unmarshal(second.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Total != 2 || p.HasMore || len(p.Events) != 1 || p.Events[0].Level != "error" {
		t.Fatal(p)
	}
	if all := c.Status(); all.EventCount != 152 || len(all.Events) != 100 {
		t.Fatal("filter mutated history", all.EventCount, len(all.Events))
	}
	for _, route := range []string{"/api/status", "/api/events"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", route+"?failures_only=invalid", nil))
		if w.Code != 400 {
			t.Fatal("invalid filter accepted", route, w.Code)
		}
	}
}

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
