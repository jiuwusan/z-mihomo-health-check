package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestNodeInfoReadsExistingMetadataWithoutDelay(t *testing.T) {
	now := time.Now().Add(-time.Minute).UTC()
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/proxies" {
			t.Errorf("unexpected side effect: %s %s", r.Method, r.URL)
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{
			"🚀 节点选择": map[string]any{"type": "Selector", "all": []string{"SG", "US", "Unknown", "Cycle", "Unprobed"}, "now": "US"},
			"SG":     map[string]any{"type": "Fallback", "now": "SG02"}, "SG02": map[string]any{"type": "Shadowsocks", "history": []any{map[string]any{"time": now.Add(-time.Minute), "delay": 100}, map[string]any{"time": now, "delay": 0}}},
			"US": map[string]any{"type": "Fallback", "now": "US01"}, "US01": map[string]any{"type": "AnyTLS", "history": []any{map[string]any{"time": now, "delay": 151}}},
			"Unknown":  map[string]any{"type": "Fallback", "now": "missing"},
			"Unprobed": map[string]any{"type": "Shadowsocks", "alive": true, "history": []any{}},
			"Cycle":    map[string]any{"type": "Selector", "now": "Cycle2"}, "Cycle2": map[string]any{"type": "Selector", "now": "Cycle"},
		}})
	}))
	defer api.Close()
	c := testController(t, testConfig(api.URL))
	cfg := c.Config()
	cfg.Enabled = false
	c.Configure(cfg)
	c.mu.Lock()
	c.state.Failures = 9
	c.state.Phase = "stopped"
	c.mu.Unlock()
	w := httptest.NewRecorder()
	NewServer(c, filepath.Join(t.TempDir(), "config.json")).ServeHTTP(w, httptest.NewRequest("GET", "/api/node-info", nil))
	var s Status
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || requests != 1 {
		t.Fatal(w.Code, requests, w.Body.String())
	}
	if s.Current != "US" || s.Failures != 9 || s.LastCheck != nil || s.Phase != "stopped" || len(s.Results) != 5 {
		t.Fatal("metadata sync changed monitoring state", s)
	}
	if s.Results[0].Name != "SG" || s.Results[0].Node != "SG02" || s.Results[0].Available || !s.Results[0].StatusKnown {
		t.Fatal(s.Results[0])
	}
	if s.Results[1].Name != "US" || s.Results[1].Node != "US01" || s.Results[1].Delay != 151 || !s.Results[1].Available || s.Results[1].Source != "mihomo" || s.Results[1].CheckedAt == nil || !s.Results[1].CheckedAt.Equal(now) {
		t.Fatal(s.Results[1])
	}
	for _, r := range s.Results[2:] {
		if r.StatusKnown || r.Available || r.CheckedAt != nil {
			t.Fatal("invented cached health", r)
		}
	}
	if c.Status().NodeInfoSyncedAt == nil || len(c.Status().Results) != 5 {
		t.Fatal("next poll would erase snapshot")
	}
}

func TestNodeInfoDoesNotRollbackConcurrentSelection(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.Write([]byte(`{"proxies":{"🚀 节点选择":{"type":"Selector","all":["US","JP"],"now":"US"},"US":{"type":"Shadowsocks"},"JP":{"type":"Shadowsocks"}}}`))
	}))
	defer api.Close()
	c := testController(t, testConfig(api.URL))
	c.update(c.rev, func(s *Status) { s.Current = "US" })
	done := make(chan error, 1)
	go func() { _, err := c.SyncNodeInfo(context.Background()); done <- err }()
	<-entered
	c.update(c.rev, func(s *Status) { s.Current = "JP" })
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if c.Status().Current != "JP" {
		t.Fatal("old metadata rolled back current selection", c.Status().Current)
	}
}

func TestNodeInfoRejectsOldConfigurationAndKeepsConcurrentProbe(t *testing.T) {
	for _, changeConfig := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_probe", true: "new_configuration"}[changeConfig], func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-release
				w.Write([]byte(`{"proxies":{"🚀 节点选择":{"type":"Selector","all":["US"],"now":"US"},"US":{"type":"Shadowsocks","alive":false}}}`))
			}))
			defer api.Close()
			c := testController(t, testConfig(api.URL))
			done := make(chan error, 1)
			go func() { _, err := c.SyncNodeInfo(context.Background()); done <- err }()
			<-entered
			if changeConfig {
				cfg := c.Config()
				cfg.Group = "Other"
				c.Configure(cfg)
			} else {
				c.mu.Lock()
				now := time.Now()
				c.state.Results = []GroupResult{{Name: "US", Available: true, Delay: 123, CheckedAt: &now}}
				c.mu.Unlock()
			}
			close(release)
			err := <-done
			if changeConfig {
				if err == nil || len(c.Status().Results) != 0 {
					t.Fatal("old metadata committed", err)
				}
			} else {
				if err != nil || !c.Status().Results[0].Available || c.Status().Results[0].Delay != 123 {
					t.Fatal("concurrent probe overwritten", err, c.Status().Results)
				}
			}
		})
	}
}
