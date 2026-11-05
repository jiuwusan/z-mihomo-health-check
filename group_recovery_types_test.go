package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type typedGroupAPI struct {
	mu                                                   sync.Mutex
	kind, selected, fixed, original, recheck, healthUsed string
	healthURL, expected, timeout                         string
	health                                               map[string]int
	falsePositive                                        string
	manualInner, manualMain, manualFixed, freezeCache    bool
	current                                              string
	probes                                               int
	puts                                                 []string
	mainTests                                            int
	probeTimeout, recheckTimeout                         string
	manualDuringMain, recheckFails                       bool
}

func typedGroupServer(t *testing.T, kind string) (*typedGroupAPI, *httptest.Server) {
	t.Helper()
	f := &typedGroupAPI{kind: kind, selected: "A1", current: "A", healthURL: "https://health.example/", expected: "204", timeout: "5000", health: map[string]int{"A1": 0, "A2": 250, "A3": 10}}
	if kind == "Selector" {
		f.expected = ""
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == "DELETE" {
			t.Error("must not unfix automatic groups")
			w.WriteHeader(400)
			return
		}
		if r.Method == "PUT" {
			var body struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			name := strings.TrimPrefix(r.URL.Path, "/proxies/")
			f.puts = append(f.puts, name+":"+body.Name)
			if name == "A" {
				f.selected = body.Name
			} else {
				f.current = body.Name
			}
			w.WriteHeader(204)
			return
		}
		switch r.URL.Path {
		case "/proxies/🚀 节点选择":
			json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "now": f.current, "all": []string{"A", "B"}})
			return
		case "/proxies/🚀 节点选择/delay":
			f.probeTimeout = r.URL.Query().Get("timeout")
			f.original = r.URL.Query().Get("url")
			w.WriteHeader(504)
			return
		case "/proxies/A":
			json.NewEncoder(w).Encode(map[string]any{"type": f.kind, "now": f.selected, "fixed": f.fixed, "all": []string{"A1", "A2", "A3"}, "testUrl": f.healthURL, "expectedStatus": f.expected})
			return
		case "/proxies/A/delay":
			f.recheckTimeout = r.URL.Query().Get("timeout")
			f.recheck = r.URL.Query().Get("url")
			if f.recheck == f.original || f.recheck == "" {
				t.Error("reused failed site")
			}
			if f.recheckFails || f.health[f.selected] <= 0 {
				w.WriteHeader(504)
				return
			}
			w.Write([]byte(`{"delay":90}`))
			return
		case "/group/A/delay":
			t.Error("select/url-test member check must not use unfixing group route")
			w.WriteHeader(500)
			return
		case "/group/🚀 节点选择/delay":
			f.mainTests++
			if f.manualDuringMain {
				f.selected = "A3"
				if f.kind == "URLTest" {
					f.fixed = "A3"
				}
			}
			json.NewEncoder(w).Encode(map[string]int{"B": 100})
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/proxies/")
		if strings.HasSuffix(name, "/delay") {
			name = strings.TrimSuffix(name, "/delay")
			if _, ok := f.health[name]; !ok {
				t.Error("unknown member", name)
				w.WriteHeader(404)
				return
			}
			f.probes++
			healthURL := r.URL.Query().Get("url")
			expected := f.expected
			if kind == "Selector" {
				expected = "200-299"
			}
			if f.healthURL != "" && healthURL != f.healthURL {
				t.Error("ignored configured health URL")
			}
			if f.healthURL == "" && healthURL == f.original {
				t.Error("selector reused failed URL")
			}
			if f.healthUsed != "" && f.healthUsed != healthURL {
				t.Error("member checks used different sites")
			}
			f.healthUsed = healthURL
			if r.URL.Query().Get("expected") != expected || r.URL.Query().Get("timeout") != f.timeout {
				t.Error("wrong member health parameters", r.URL.Query())
			}
			if f.manualMain {
				f.current = "B"
			}
			if f.manualInner {
				f.selected = "A3"
			}
			if f.manualFixed {
				f.fixed = "A3"
				f.selected = "A3"
			}
			if kind == "URLTest" && f.probes == 3 && !f.freezeCache && f.fixed == "" {
				f.selected = "A3"
			}
			if f.health[name] <= 0 {
				w.WriteHeader(504)
				return
			}
			json.NewEncoder(w).Encode(map[string]int{"delay": f.health[name]})
			return
		}
		if delay, ok := f.health[name]; ok {
			alive := delay > 0 && f.falsePositive != name
			if !alive {
				delay = 0
			}
			json.NewEncoder(w).Encode(map[string]any{"type": "Shadowsocks", "extra": map[string]any{f.healthUsed: map[string]any{"alive": alive, "history": []any{map[string]any{"time": time.Now().UTC(), "delay": delay}}}}})
			return
		}
		t.Error("unexpected request", r.Method, r.URL)
		w.WriteHeader(404)
	}))
	t.Cleanup(s.Close)
	return f, s
}

func TestSelectorRecoveryOnlySwitchesFailedMemberInOriginalOrder(t *testing.T) {
	for _, healthyCurrent := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch", true: "keep"}[healthyCurrent], func(t *testing.T) {
			f, api := typedGroupServer(t, "Selector")
			if healthyCurrent {
				f.health["A1"] = 100
			}
			c := testFallbackController(t, testConfig(api.URL))
			c.Check(context.Background())
			if f.probes != 3 || f.mainTests != 0 || c.Status().Current != "A" {
				t.Fatal("selector did not repair inside A", f.probes, f.mainTests, f.puts)
			}
			if healthyCurrent {
				if len(f.puts) != 0 || f.selected != "A1" {
					t.Fatal("healthy manual selection replaced", f.puts)
				}
			} else {
				if len(f.puts) != 1 || f.puts[0] != "A:A2" {
					t.Fatal("did not follow original order", f.puts)
				}
			}
			if f.recheck == "" || f.recheck == f.original {
				t.Fatal("missing independent exit recheck")
			}
		})
	}
}

func TestSelectorRecoveryUsesRandomHealthSiteAndRejectsFalsePositiveDelay(t *testing.T) {
	f, api := typedGroupServer(t, "Selector")
	f.healthURL = ""
	f.falsePositive = "A2"
	cfg := testConfig(api.URL)
	cfg.TimeoutSeconds = 8
	c := testFallbackController(t, cfg)
	c.Check(context.Background())
	if f.healthUsed == "" || f.healthUsed == f.original || f.selected != "A3" {
		t.Fatal("health fallback or expected-status validation failed", f.healthUsed, f.original, f.puts)
	}
}

func TestURLTestRecoveryLeavesAutomaticSelectionToMihomo(t *testing.T) {
	f, api := typedGroupServer(t, "URLTest")
	c := testFallbackController(t, testConfig(api.URL))
	c.Check(context.Background())
	if f.probes != 3 || f.mainTests != 0 || len(f.puts) != 0 || f.selected != "A3" || c.Status().Failures != 0 {
		t.Fatal("url-test was not recovered automatically", f.probes, f.mainTests, f.puts, c.Status())
	}
}

func TestTypedGroupRecoveryProtectsManualChanges(t *testing.T) {
	for _, mode := range []string{"selector_inner", "selector_main", "urltest_fixed"} {
		t.Run(mode, func(t *testing.T) {
			kind := "Selector"
			if mode == "urltest_fixed" {
				kind = "URLTest"
			}
			f, api := typedGroupServer(t, kind)
			f.manualInner = mode == "selector_inner"
			f.manualMain = mode == "selector_main"
			f.manualFixed = mode == "urltest_fixed"
			c := testFallbackController(t, testConfig(api.URL))
			c.Check(context.Background())
			if len(f.puts) != 0 || f.mainTests != 0 || f.recheck != "" {
				t.Fatal("manual operation overwritten", f.puts, f.mainTests, f.recheck)
			}
		})
	}
}

func TestURLTestRecoveryDoesNotUnfixExistingSelection(t *testing.T) {
	f, api := typedGroupServer(t, "URLTest")
	f.fixed = "A1"
	c := testFallbackController(t, testConfig(api.URL))
	c.Check(context.Background())
	if f.fixed != "A1" || f.probes != 0 {
		t.Fatal("existing manual pin changed", f.fixed, f.probes)
	}
	for _, put := range f.puts {
		if strings.HasPrefix(put, "A:") {
			t.Fatal("forced url-test member", put)
		}
	}
}

func TestURLTestCacheWaitCancelledByConfiguration(t *testing.T) {
	f, api := typedGroupServer(t, "URLTest")
	f.freezeCache = true
	c := testFallbackController(t, testConfig(api.URL))
	done := make(chan struct{})
	go func() { defer close(done); c.Check(context.Background()) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(c.Status().Message, "缓存") {
		select {
		case <-done:
			t.Fatal("did not wait for routing cache")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("cache wait never began")
		}
		time.Sleep(time.Millisecond)
	}
	cfg := c.Config()
	cfg.Group = "Other"
	c.Configure(cfg)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cache wait ignored cancellation")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.puts) != 0 || f.mainTests != 0 {
		t.Fatal("cancelled cache wait continued", f.puts, f.mainTests)
	}
}

func TestTypedGroupProtectsManualChangeDuringMainDelay(t *testing.T) {
	for _, kind := range []string{"Selector", "URLTest"} {
		t.Run(kind, func(t *testing.T) {
			f, api := typedGroupServer(t, kind)
			f.manualDuringMain, f.recheckFails = true, true
			c := testFallbackController(t, testConfig(api.URL))
			c.Check(context.Background())
			if f.mainTests != 1 || f.current != "A" {
				t.Fatal("manual inner change was overridden by main failover", f.current, f.puts)
			}
		})
	}
}

func TestCountryHealthTimeoutIndependentOfProbeConfig(t *testing.T) {
	for _, tc := range []struct {
		seconds int
		query   string
	}{{1, "1000"}, {8, "8000"}, {32, "32000"}} {
		for _, kind := range []string{"Fallback", "Selector", "URLTest"} {
			t.Run(kind+"_"+tc.query, func(t *testing.T) {
				var api *httptest.Server
				var original, recheck *string
				if kind == "Fallback" {
					f, s := fallbackServer(t)
					api, original, recheck = s, &f.probeTimeout, &f.recheckTimeout
				} else {
					f, s := typedGroupServer(t, kind)
					api, original, recheck = s, &f.probeTimeout, &f.recheckTimeout
				}
				cfg := testConfig(api.URL)
				cfg.TimeoutSeconds = tc.seconds
				c := testFallbackController(t, cfg)
				c.Check(context.Background())
				if *original != tc.query || *recheck != tc.query || c.Status().Phase != "healthy" {
					t.Fatalf("regular/recheck timeout changed or recovery failed: %s/%s, %+v", *original, *recheck, c.Status())
				}
			})
		}
	}
}
