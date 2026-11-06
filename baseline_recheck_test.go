package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBaselineRecheckBeforeRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, healthURL, expected, wantURL, wantExpected string
		codes                                            []int
		wantAttempts                                     int
		wantRecovery                                     bool
	}{
		{"first_success", "https://health.example/", "*", "https://health.example/", "*", []int{200}, 1, false},
		{"second_success", "https://health.example/", "204", "https://health.example/", "204", []int{503, 200}, 2, false},
		{"third_success", "https://health.example/", "", "https://health.example/", "", []int{503, 504, 200}, 3, false},
		{"all_failed", "https://health.example/", "204", "https://health.example/", "204", []int{503, 504, 503}, 3, true},
		{"default_url", "", "*", "http://www.gstatic.com/generate_204", "204", []int{200}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts, inner, batches, puts := 0, 0, 0, 0
			current := "A"
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/proxies/🚀 节点选择":
					if r.Method == "PUT" {
						puts++
						current = "B"
						w.WriteHeader(204)
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "now": current, "all": []string{"A", "B"}})
				case "/proxies/🚀 节点选择/delay":
					w.WriteHeader(503)
				case "/proxies/A":
					json.NewEncoder(w).Encode(map[string]any{"type": "Fallback", "now": "A1", "all": []string{"A1"}, "testUrl": tc.healthURL, "expectedStatus": tc.expected})
				case "/proxies/A/delay":
					if r.URL.Query().Get("url") != tc.wantURL {
						w.WriteHeader(503)
						return
					}
					if r.URL.Query().Get("expected") != tc.wantExpected || r.URL.Query().Get("timeout") != "2000" {
						t.Errorf("wrong baseline query: %v", r.URL.Query())
					}
					attempts++
					if attempts > len(tc.codes) {
						t.Error("unexpected extra baseline attempt")
						w.WriteHeader(503)
						return
					}
					code := tc.codes[attempts-1]
					w.WriteHeader(code)
					if code == 200 {
						w.Write([]byte(`{"delay":87}`))
					}
				case "/group/A/delay":
					inner++
					if attempts != 3 {
						t.Error("recovery before three baseline failures")
					}
					json.NewEncoder(w).Encode(map[string]int{"A1": 100})
				case "/group/🚀 节点选择/delay":
					batches++
					if r.URL.Query().Get("url") == tc.wantURL {
						t.Error("main test lost original random URL")
					}
					json.NewEncoder(w).Encode(map[string]int{"B": 100})
				default:
					t.Errorf("unexpected request: %s", r.URL)
					w.WriteHeader(404)
				}
			}))
			defer api.Close()
			cfg := testConfig(api.URL)
			cfg.FailureThreshold = 3
			cfg.TimeoutSeconds = 2
			c := testFallbackController(t, cfg)
			c.Check(context.Background())
			c.Check(context.Background())
			if attempts != 0 || inner != 0 || c.Status().Failures != 2 {
				t.Fatal("baseline ran before failure threshold")
			}
			c.Check(context.Background())
			if attempts != tc.wantAttempts {
				t.Fatalf("baseline attempts = %d, want %d", attempts, tc.wantAttempts)
			}
			s := c.Status()
			if tc.wantRecovery {
				if inner != 1 || batches != 1 || puts != 1 || current != "B" {
					t.Fatalf("did not resume existing recovery: inner=%d batches=%d puts=%d current=%s", inner, batches, puts, current)
				}
			} else {
				if inner != 0 || batches != 0 || puts != 0 || s.Current != "A" || s.Failures != 0 || s.Delay != 87 || s.LastSite != tc.wantURL || s.LastSwitch != nil {
					t.Fatalf("successful baseline did not retain current exit: %+v", s)
				}
			}
			if c.pool.states["https://test.example/"].Failures+c.pool.states["https://alternate.example/"].Failures != 3 {
				t.Fatal("baseline changed independent site counters")
			}
		})
	}
}

func TestBaselineRecheckUsesGroupExpectedAndDefaultForPlainNode(t *testing.T) {
	for _, tc := range []struct{ kind, url, expected, wantURL, wantExpected string }{
		{"Selector", "https://health.example/", "", "https://health.example/", "200-299"},
		{"Selector", "https://health.example/", "*", "https://health.example/", "*"},
		{"URLTest", "https://health.example/", "", "https://health.example/", ""},
		{"Shadowsocks", "", "", "http://www.gstatic.com/generate_204", "204"},
	} {
		t.Run(tc.kind+tc.expected, func(t *testing.T) {
			f, api := fallbackServer(t)
			f.kind, f.healthURL, f.expected = tc.kind, tc.url, tc.expected
			calls := 0
			wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/proxies/A/delay" {
					calls++
					if r.URL.Query().Get("url") != tc.wantURL || r.URL.Query().Get("expected") != tc.wantExpected {
						t.Errorf("wrong baseline parameters: %v", r.URL.Query())
					}
					w.Write([]byte(`{"delay":75}`))
					return
				}
				api.Config.Handler.ServeHTTP(w, r)
			}))
			defer wrapped.Close()
			c := testFallbackController(t, testConfig(wrapped.URL))
			c.Check(context.Background())
			if calls != 1 || c.Status().Failures != 0 || c.Status().Delay != 75 || !reflect.DeepEqual(f.trace, []string{"probe"}) {
				t.Fatal("did not stop after baseline success", calls, f.trace, c.Status())
			}
		})
	}
}

func TestBaselineRecheckCancellationStopsInFlightRequest(t *testing.T) {
	f, api := fallbackServer(t)
	entered := make(chan struct{})
	wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxies/A/delay" {
			close(entered)
			<-r.Context().Done()
			return
		}
		api.Config.Handler.ServeHTTP(w, r)
	}))
	defer wrapped.Close()
	c := testFallbackController(t, testConfig(wrapped.URL))
	done := make(chan struct{})
	go func() { defer close(done); c.Check(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("baseline request never began")
	}
	c.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("configuration cancellation did not interrupt baseline request")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.trace, []string{"probe"}) || c.Status().Running {
		t.Fatal("cancelled cycle continued", f.trace, c.Status())
	}
}

func TestBaselineRecheckStopsOnControlErrorOrSelectionChange(t *testing.T) {
	for _, mode := range []string{"auth", "invalid_json", "invalid_url", "manual_main", "manual_inner", "selector_choice", "changed_url", "fixed", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f, api := fallbackServer(t)
			if mode == "selector_choice" {
				f.kind = "Selector"
			}
			if mode == "invalid_url" {
				f.healthURL = "ftp://health.example/"
			}
			// Wrap the existing fake at the HTTP boundary to distinguish baseline
			// requests from the later random-site recheck.
			original := api.Config.Handler
			calls := 0
			var c *Controller
			wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/proxies/A/delay" && r.URL.Query().Get("url") == "https://health.example/generate_204" {
					calls++
					switch mode {
					case "auth":
						w.WriteHeader(401)
					case "invalid_json":
						w.Write([]byte(`{`))
					case "manual_main":
						f.mu.Lock()
						f.current = "B"
						f.mu.Unlock()
						w.Write([]byte(`{"delay":80}`))
					case "cancel":
						c.Stop()
						w.WriteHeader(503)
					default:
						w.WriteHeader(503)
					}
					return
				}
				if calls > 0 && r.URL.Path == "/proxies/A" && (mode == "manual_inner" || mode == "selector_choice" || mode == "changed_url" || mode == "fixed") {
					data := map[string]any{"type": f.kind, "now": "A1", "all": []string{"A1", "A2"}, "testUrl": f.healthURL, "expectedStatus": f.expected}
					if mode == "manual_inner" {
						data["all"] = []string{"A2", "A1"}
					}
					if mode == "selector_choice" {
						data["now"] = "A2"
					}
					if mode == "changed_url" {
						data["testUrl"] = "https://changed.example/"
					}
					if mode == "fixed" {
						data["fixed"] = "A1"
					}
					json.NewEncoder(w).Encode(data)
					return
				}
				original.ServeHTTP(w, r)
			}))
			defer wrapped.Close()
			c = testFallbackController(t, testConfig(wrapped.URL))
			c.Check(context.Background())
			wantCalls := 1
			if mode == "invalid_url" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("expected %d baseline requests, got %d", wantCalls, calls)
			}
			if !reflect.DeepEqual(f.trace, []string{"probe"}) {
				t.Fatal("continued old recovery", f.trace)
			}
			if mode == "manual_main" && c.Status().Current != "B" {
				t.Fatal("manual selection lost")
			}
			if mode == "auth" || mode == "invalid_json" || mode == "invalid_url" {
				if c.Status().Phase != "error" || c.Status().Failures != 1 {
					t.Fatal("control error treated as baseline node failure", c.Status())
				}
			}
			for _, e := range c.Status().Events {
				if strings.Contains(e.Message, "test-secret") {
					t.Fatal("secret leaked")
				}
			}
		})
	}
}
