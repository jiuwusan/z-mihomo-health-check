package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

type fallbackAPI struct {
	failedSite, verifySite                    string
	probeTimeout, recheckTimeout              string
	order                                     []string
	manualMain                                bool
	mu                                        sync.Mutex
	current, kind, healthURL, expected        string
	recovered, manualSwitch                   bool
	metadataStatus, innerStatus, verifyStatus int
	trace                                     []string
	block, entered                            chan struct{}
}

func fallbackServer(t *testing.T) (*fallbackAPI, *httptest.Server) {
	t.Helper()
	f := &fallbackAPI{current: "A", kind: "Fallback", healthURL: "https://health.example/generate_204", expected: "204", recovered: true, order: []string{"A", "B"}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == "PUT" {
			var body struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.trace = append(f.trace, "select:"+body.Name)
			f.current = body.Name
			w.WriteHeader(204)
			return
		}
		switch r.URL.Path {
		case "/proxies/🚀 节点选择":
			json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": f.order, "now": f.current})
		case "/proxies/🚀 节点选择/delay":
			f.probeTimeout = r.URL.Query().Get("timeout")
			f.trace = append(f.trace, "probe")
			f.failedSite = r.URL.Query().Get("url")
			w.WriteHeader(504)
		case "/proxies/A":
			if f.metadataStatus != 0 {
				w.WriteHeader(f.metadataStatus)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"type": f.kind, "all": []string{"A1", "A2"}, "now": "A1", "testUrl": f.healthURL, "expectedStatus": f.expected})
		case "/group/A/delay":
			f.trace = append(f.trace, "inner")
			if r.URL.Query().Get("url") != f.healthURL || r.URL.Query().Get("expected") != f.expected || r.URL.Query().Get("timeout") != "5000" {
				t.Errorf("health query differs: %v", r.URL.Query())
			}
			if f.block != nil {
				close(f.entered)
				block := f.block
				f.mu.Unlock()
				select {
				case <-block:
				case <-r.Context().Done():
				}
				f.mu.Lock()
				return
			}
			if f.manualSwitch {
				f.current = "B"
			}
			if f.innerStatus != 0 {
				w.WriteHeader(f.innerStatus)
				return
			}
			json.NewEncoder(w).Encode(map[string]int{"A2": 100})
		case "/proxies/A/delay":
			// These tests exercise the recovery flow after baseline confirmation.
			if site := r.URL.Query().Get("url"); site == f.healthURL || site == "http://www.gstatic.com/generate_204" {
				w.WriteHeader(503)
				return
			}
			f.recheckTimeout = r.URL.Query().Get("timeout")
			f.trace = append(f.trace, "verify")
			f.verifySite = r.URL.Query().Get("url")
			if f.verifySite == "" || f.verifySite == f.failedSite || r.URL.Query().Get("expected") != "200-299" {
				t.Errorf("wrong exit recheck query: %v", r.URL.Query())
			}
			if f.verifyStatus != 0 {
				w.WriteHeader(f.verifyStatus)
				return
			}
			if !f.recovered {
				w.WriteHeader(504)
				return
			}
			w.Write([]byte(`{"delay":90}`))
		case "/group/🚀 节点选择/delay":
			f.trace = append(f.trace, "main")
			if f.manualMain {
				f.current = "C"
			}
			json.NewEncoder(w).Encode(map[string]int{"B": 100})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(s.Close)
	return f, s
}

func testFallbackController(t *testing.T, cfg Config) *Controller {
	t.Helper()
	c := NewController([]string{"https://test.example/", "https://alternate.example/"})
	c.Configure(cfg)
	t.Cleanup(c.Stop)
	return c
}

func TestFallbackSelfRecoveryRunsAtThresholdBeforeMainAndRetainsA(t *testing.T) {
	for _, expected := range []string{"204", ""} {
		t.Run("expected_"+expected, func(t *testing.T) {
			f, api := fallbackServer(t)
			f.expected = expected
			cfg := testConfig(api.URL)
			cfg.FailureThreshold = 3
			c := testFallbackController(t, cfg)
			c.Check(context.Background())
			c.Check(context.Background())
			if !reflect.DeepEqual(f.trace, []string{"probe", "probe"}) {
				t.Fatal("early recovery", f.trace)
			}
			c.Check(context.Background())
			if !reflect.DeepEqual(f.trace, []string{"probe", "probe", "probe", "inner", "verify"}) {
				t.Fatal("wrong recovery order", f.trace)
			}
			s := c.Status()
			if s.Current != "A" || s.Failures != 0 || s.Delay != 90 || s.Phase != "healthy" || s.LastSwitch != nil {
				t.Fatal("did not retain recovered A", s)
			}
			if c.pool.states[f.failedSite].Failures == 0 {
				t.Fatal("different site's success cleared original failed site")
			}
			if s.LastSite != f.verifySite {
				t.Fatal("panel still displays failed site for successful recheck", s.LastSite, f.verifySite)
			}
		})
	}
}

func TestFallbackSelfRecoveryRecheckSkipsQuarantinedSites(t *testing.T) {
	f, api := fallbackServer(t)
	cfg := testConfig(api.URL)
	c := NewController([]string{"https://test.example/", "https://alternate.example/", "https://isolated.example/"})
	c.Configure(cfg)
	defer c.Stop()
	url := "https://isolated.example/"
	token := failSite(t, c.pool, url, time.Now())
	if _, err := c.pool.FinishReview(url, token, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	c.Check(context.Background())
	if f.verifySite == "" || f.verifySite == url || f.verifySite == f.failedSite {
		t.Fatal("invalid recheck site", f.failedSite, f.verifySite)
	}
}

func TestFallbackSelfRecoveryDoesNotReuseSiteWhenNoAlternative(t *testing.T) {
	f, api := fallbackServer(t)
	c := testController(t, testConfig(api.URL))
	c.Check(context.Background())
	if !reflect.DeepEqual(f.trace, []string{"probe", "inner", "main", "select:B"}) {
		t.Fatal("reused original site without an alternative", f.trace)
	}
}

func TestFallbackSelfRecoveryContinuesMainOnlyAfterFailedRecheck(t *testing.T) {
	f, api := fallbackServer(t)
	f.recovered = false
	f.innerStatus = 504
	c := testFallbackController(t, testConfig(api.URL))
	c.Check(context.Background())
	if !reflect.DeepEqual(f.trace, []string{"probe", "inner", "verify", "main", "select:B"}) {
		t.Fatal("wrong fallback/main order", f.trace)
	}
	if c.Status().Current != "B" {
		t.Fatal(c.Status())
	}
}

func TestFallbackSelfRecoverySkipsUnsupportedOrMissingURL(t *testing.T) {
	for _, kind := range []string{"LoadBalance", "missing_url"} {
		t.Run(kind, func(t *testing.T) {
			f, api := fallbackServer(t)
			if kind == "missing_url" {
				f.healthURL = ""
			} else {
				f.kind = kind
			}
			c := testFallbackController(t, testConfig(api.URL))
			c.Check(context.Background())
			if !reflect.DeepEqual(f.trace, []string{"probe", "main", "select:B"}) {
				t.Fatal(f.trace)
			}
		})
	}
}

func TestFallbackSelfRecoveryStopsOnControlErrorsOrManualSwitch(t *testing.T) {
	for _, mode := range []string{"metadata_auth", "metadata_missing", "inner_api", "verify_auth", "manual"} {
		t.Run(mode, func(t *testing.T) {
			f, api := fallbackServer(t)
			switch mode {
			case "metadata_auth":
				f.metadataStatus = 401
			case "metadata_missing":
				f.metadataStatus = 404
			case "inner_api":
				f.innerStatus = 503
			case "verify_auth":
				f.verifyStatus = 401
			case "manual":
				f.manualSwitch = true
			}
			c := testFallbackController(t, testConfig(api.URL))
			c.Check(context.Background())
			for _, v := range f.trace {
				if v == "main" || v == "select:B" {
					t.Fatal("continued stale/control-error cycle", f.trace)
				}
			}
			if mode == "manual" && c.Status().Current != "B" {
				t.Fatal("manual selection lost")
			}
		})
	}
}

func TestFallbackSelfRecoveryDoesNotOverrideManualSelectionDuringMainProbe(t *testing.T) {
	f, api := fallbackServer(t)
	f.recovered = false
	f.manualMain = true
	f.order = []string{"A", "B", "C"}
	c := testFallbackController(t, testConfig(api.URL))
	c.Check(context.Background())
	if f.current != "C" || c.Status().Current != "C" {
		t.Fatal("main probe overwrote manual C selection", f.trace, c.Status().Current)
	}
	for _, v := range f.trace {
		if v == "select:B" {
			t.Fatal("automatic PUT after manual switch", f.trace)
		}
	}
}

func TestFallbackSelfRecoveryCancelledByConfigurationChange(t *testing.T) {
	f, api := fallbackServer(t)
	f.block = make(chan struct{})
	f.entered = make(chan struct{})
	cfg := testConfig(api.URL)
	c := testFallbackController(t, cfg)
	done := make(chan struct{})
	go func() { defer close(done); c.Check(context.Background()) }()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("inner probe never began")
	}
	cfg.Group = "Other"
	c.Configure(cfg)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("old fallback probe not cancelled")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.trace {
		if v == "main" || v == "select:B" {
			t.Fatal("old cycle switched", f.trace)
		}
	}
}
