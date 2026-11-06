package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/gateway"
)

// A real HTTP fake catches authentication, routing and side effects together.
type fakeMihomo struct {
	mu              sync.Mutex
	probeStatus     int
	groupStatus     int
	delays          map[string]int
	current         string
	order           []string
	probes, batches int
	selections      []string
	reject          map[string]bool
	block           chan struct{}
}

func newFake(t *testing.T) (*fakeMihomo, *httptest.Server) {
	t.Helper()
	f := &fakeMihomo{probeStatus: 504, groupStatus: 200, current: "Japan", order: []string{"US", "Japan", "Singapore"}, delays: map[string]int{"US": 900, "Japan": 100}, reject: map[string]bool{}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/proxies":
			json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{"🚀 节点选择": map[string]any{"type": "Selector", "all": f.order, "now": f.current}}})
		case strings.HasPrefix(r.URL.Path, "/group/") && strings.HasSuffix(r.URL.Path, "/delay"):
			f.batches++
			if r.URL.Query().Get("expected") != "200-299" || r.URL.Query().Get("url") != "https://test.example/" || r.URL.Query().Get("timeout") != "4000" {
				t.Errorf("wrong query: %v", r.URL.Query())
			}
			w.WriteHeader(f.groupStatus)
			json.NewEncoder(w).Encode(f.delays)
		case r.Method == "PUT":
			var body struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.selections = append(f.selections, body.Name)
			if f.reject[body.Name] {
				w.WriteHeader(503)
				return
			}
			f.current = body.Name
			w.WriteHeader(204)
		case strings.HasPrefix(r.URL.Path, "/proxies/") && !strings.HasSuffix(r.URL.Path, "/delay") && r.URL.Path != "/proxies/🚀 节点选择":
			json.NewEncoder(w).Encode(map[string]any{"type": "Shadowsocks"})
		case strings.HasSuffix(r.URL.Path, "/delay"):
			f.probes++
			if f.block != nil {
				block := f.block
				f.mu.Unlock()
				select {
				case <-block:
				case <-r.Context().Done():
				}
				f.mu.Lock()
			}
			w.WriteHeader(f.probeStatus)
			if f.probeStatus == 200 {
				fmt.Fprint(w, `{"delay":123}`)
			}
		default:
			if r.URL.Path != "/proxies/🚀 节点选择" {
				t.Errorf("bad escaped group: %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": f.order, "now": f.current})
		}
	}))
	t.Cleanup(s.Close)
	return f, s
}
func testConfig(base string) config.Config {
	c := config.DefaultConfig()
	c.APIURL, c.Secret, c.Enabled = base, "test-secret", true
	return c
}
func testController(t *testing.T, cfg config.Config) *Controller {
	t.Helper()
	c := NewController([]string{"https://test.example/"})
	c.Configure(cfg)
	t.Cleanup(c.Stop)
	return c
}
func TestFailoverUsesOriginalOrderNotDelay(t *testing.T) {
	f, s := newFake(t)
	c := testController(t, testConfig(s.URL))
	c.Check(context.Background())
	if f.current != "US" || f.batches != 1 || len(f.selections) != 1 {
		t.Fatalf("wanted US despite slower delay: %+v", f)
	}
	status := c.Status()
	if status.Failures != 0 || status.Current != "US" {
		t.Fatalf("bad state: %+v", status)
	}
	for _, result := range status.Results {
		if result.CheckedAt == nil || result.CheckedAt.After(time.Now()) {
			t.Fatalf("missing batch result time: %+v", result)
		}
	}
}

func TestProbeFailureRecordsSiteSelectedGroupTimeoutAndAPICode(t *testing.T) {
	for _, code := range []int{503, 504} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			f, api := newFake(t)
			f.probeStatus = code
			cfg := testConfig(api.URL)
			cfg.FailureThreshold = 3
			c := testController(t, cfg)
			c.Check(context.Background())
			status := c.Status()
			if status.Failures != 1 || len(status.Events) != 1 {
				t.Fatal("unexpected failure state")
			}
			for _, detail := range []string{"当前出口探查失败", fmt.Sprint(code), "Japan", "https://test.example/", "4s"} {
				if !strings.Contains(status.Events[0].Message, detail) {
					t.Fatalf("missing %q in diagnostic: %s", detail, status.Events[0].Message)
				}
			}
			if strings.Contains(status.Events[0].Message, cfg.Secret) || strings.Contains(status.Events[0].Message, cfg.APIURL) {
				t.Fatal("diagnostic leaked controller connection details")
			}
		})
	}
}
func TestProbeSuccessRecordsTestedSite(t *testing.T) {
	f, api := newFake(t)
	f.probeStatus = 200
	c := testController(t, testConfig(api.URL))
	c.Check(context.Background())
	status := c.Status()
	if len(status.Events) != 1 || status.Events[0].Level != "success" {
		t.Fatalf("expected successful probe event: %+v", status.Events)
	}
	if !strings.Contains(status.Events[0].Message, "https://test.example/") {
		t.Fatalf("success event omitted tested site: %s", status.Events[0].Message)
	}
}

func TestThresholdAndSuccessReset(t *testing.T) {
	f, s := newFake(t)
	cfg := testConfig(s.URL)
	cfg.FailureThreshold = 2
	c := testController(t, cfg)
	c.Check(context.Background())
	if f.batches != 0 || c.Status().Failures != 1 {
		t.Fatal("triggered too early")
	}
	f.probeStatus = 200
	c.Check(context.Background())
	if c.Status().Failures != 0 {
		t.Fatal("success did not reset failures")
	}
	f.probeStatus = 504
	c.Check(context.Background())
	if f.batches != 0 {
		t.Fatal("stale failure count")
	}
	c.Check(context.Background())
	if f.batches != 1 || f.current != "US" {
		t.Fatal("threshold did not trigger")
	}
}
func TestUnavailableAndZeroDelayDoNotSwitch(t *testing.T) {
	f, s := newFake(t)
	f.delays = map[string]int{"US": 0, "Japan": 0, "unrelated": 1}
	c := testController(t, testConfig(s.URL))
	c.Check(context.Background())
	if len(f.selections) != 0 || f.current != "Japan" {
		t.Fatal("switched to unavailable/unrelated member")
	}
}
func TestSwitchFailureTriesNextOriginalMember(t *testing.T) {
	f, s := newFake(t)
	f.current = "Singapore"
	f.reject["US"] = true
	c := testController(t, testConfig(s.URL))
	c.Check(context.Background())
	if f.current != "Japan" || strings.Join(f.selections, ",") != "US,Japan" {
		t.Fatalf("bad switch fallback: %v", f.selections)
	}
}
func TestAuthenticationFailureDoesNotTriggerGroupTest(t *testing.T) {
	f, s := newFake(t)
	cfg := testConfig(s.URL)
	cfg.Secret = "wrong"
	c := testController(t, cfg)
	c.Check(context.Background())
	if f.batches != 0 || c.Status().Failures != 0 {
		t.Fatal("API auth failure counted as node failure")
	}
	if strings.Contains(fmt.Sprint(c.Status()), "wrong") {
		t.Fatal("secret exposed")
	}
}
func TestConfigurationChangeCancelsInFlightAndPreventsSwitch(t *testing.T) {
	f, s := newFake(t)
	f.block = make(chan struct{})
	cfg := testConfig(s.URL)
	c := testController(t, cfg)
	done := make(chan struct{})
	go func() { c.Check(context.Background()); close(done) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		f.mu.Lock()
		n := f.probes
		f.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("probe did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cfg.Enabled = false
	c.Configure(cfg)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("old request not cancelled")
	}
	if f.batches != 0 || len(f.selections) > 0 || c.Status().Running {
		t.Fatal("disabled controller continued old cycle")
	}
}
func TestMihomoDoesNotForwardSecretOnRedirect(t *testing.T) {
	leaked := false
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer dest.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 302) }))
	defer source.Close()
	client := gateway.NewMihomo(testConfig(source.URL))
	_, err := client.Group(context.Background())
	if err == nil || leaked {
		t.Fatal("redirect followed with API credentials")
	}
}

func TestFastExitFailure503TriggersFailover(t *testing.T) {
	f, s := newFake(t)
	f.probeStatus = 503
	c := testController(t, testConfig(s.URL))
	c.Check(context.Background())
	if f.batches != 1 || f.current != "US" {
		t.Fatal("Mihomo HTTP 503 exit failure did not trigger failover")
	}
}

func TestConcurrentChecksNeverOverlap(t *testing.T) {
	f, s := newFake(t)
	f.block = make(chan struct{})
	c := testController(t, testConfig(s.URL))
	done := make(chan struct{})
	go func() { c.Check(context.Background()); close(done) }()
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		n := f.probes
		f.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("probe did not start")
		}
		time.Sleep(time.Millisecond)
	}
	c.Check(context.Background())
	f.mu.Lock()
	n := f.probes
	f.mu.Unlock()
	if n != 1 {
		t.Fatal("overlapping checks started")
	}
	c.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("check did not cancel")
	}
}
