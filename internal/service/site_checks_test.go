package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/gateway"
)

func runSiteOutcome(c *Controller, ctx context.Context, cfg config.Config, rev uint64, site string, success bool) {
	if token := c.recordSiteOutcome(ctx, rev, site, success); token != 0 {
		c.reviewSite(ctx, cfg, rev, site, token)
	}
}

func siteCheckAPI(t *testing.T, codes map[string]int) (*httptest.Server, *[]string) {
	t.Helper()
	requests := []string{}
	var mu sync.Mutex
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") {
			url := r.URL.Query().Get("url")
			mu.Lock()
			requests = append(requests, url)
			code := codes[url]
			mu.Unlock()
			if code == 0 {
				code = 200
			}
			w.WriteHeader(code)
			if code == 200 {
				w.Write([]byte(`{"delay":100}`))
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": []string{"US"}, "now": "US"})
	}))
	t.Cleanup(s.Close)
	return s, &requests
}

func TestSiteChecksNoExtraProbeBeforeTwentyAndControlErrorsIgnored(t *testing.T) {
	f, api := newFake(t)
	c := testController(t, testConfig(api.URL))
	cfg := c.Config()
	cfg.FailureThreshold = 100
	c.Configure(cfg)
	for i := 0; i < 19; i++ {
		c.Check(context.Background())
	}
	if f.probes != 19 || f.batches != 0 {
		t.Fatal("extra probe before threshold", f.probes, f.batches)
	}
	if c.pool.states["https://test.example/"].Failures != 19 {
		t.Fatal("missing count")
	}
	f.probeStatus = 401
	c.Check(context.Background())
	if c.pool.states["https://test.example/"].Failures != 19 {
		t.Fatal("authentication counted")
	}
	f.probeStatus = 200
	c.Check(context.Background())
	if c.pool.states["https://test.example/"].Failures != 0 {
		t.Fatal("success did not reset")
	}
}

func TestSiteChecksReviewOnlyAtThresholdAndExitFailureDoesNotQuarantine(t *testing.T) {
	for _, healthyControls := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired_exit", true: "bad_site"}[healthyControls], func(t *testing.T) {
			codes := map[string]int{"https://a.example/": 504}
			if !healthyControls {
				codes["https://b.example/"] = 504
				codes["https://c.example/"] = 504
			}
			api, requests := siteCheckAPI(t, codes)
			p := testSitePool(t, "")
			c := NewControllerWithPool(p)
			c.Configure(testConfig(api.URL))
			defer c.Stop()
			for i := 0; i < 19; i++ {
				runSiteOutcome(c, context.Background(), c.Config(), c.rev, "https://a.example/", false)
			}
			if len(*requests) != 0 {
				t.Fatal("review before 20", *requests)
			}
			runSiteOutcome(c, context.Background(), c.Config(), c.rev, "https://a.example/", false)
			active, _ := p.Counts()
			if healthyControls {
				if active != 2 || len(*requests) != 3 {
					t.Fatal(active, *requests)
				}
			} else {
				if active != 3 || len(*requests) != 1 {
					t.Fatal(active, *requests)
				}
			}
			before := len(*requests)
			runSiteOutcome(c, context.Background(), c.Config(), c.rev, "https://a.example/", false)
			if len(*requests) != before {
				t.Fatal("cooldown/isolation bypassed")
			}
		})
	}
}

func TestSiteChecksCancelledAndStaleOutcomeIgnored(t *testing.T) {
	p := testSitePool(t, "")
	c := NewControllerWithPool(p)
	cfg := testConfig("http://127.0.0.1:1")
	c.Configure(cfg)
	defer c.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runSiteOutcome(c, ctx, cfg, c.rev, "https://a.example/", false)
	old := c.rev
	c.Configure(cfg)
	runSiteOutcome(c, context.Background(), cfg, old, "https://a.example/", false)
	if len(p.states) != 0 {
		t.Fatal("old/cancelled outcome counted")
	}
}

func TestSiteChecksEmptyPoolCanRecover(t *testing.T) {
	p := testSitePool(t, "")
	past := time.Now().Add(-2 * time.Hour)
	for _, url := range p.sites {
		token := failSite(t, p, url, past)
		if _, err := p.FinishReview(url, token, true, past); err != nil {
			t.Fatal(err)
		}
	}
	api, _ := siteCheckAPI(t, map[string]int{})
	c := NewControllerWithPool(p)
	c.Configure(testConfig(api.URL))
	defer c.Stop()
	if c.Status().PoolSize != 0 {
		t.Fatal("pool not empty")
	}
	c.Check(context.Background())
	if c.Status().PoolSize != 1 {
		t.Fatal("empty pool could not revive", c.Status())
	}
}

func TestSiteChecksConcurrentSuccessInvalidatesReview(t *testing.T) {
	p := testSitePool(t, "")
	url := "https://a.example/"
	entered, release := make(chan struct{}), make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") {
			if r.URL.Query().Get("url") == url {
				close(entered)
				<-release
				w.WriteHeader(504)
			} else {
				w.Write([]byte(`{"delay":100}`))
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": []string{"US"}, "now": "US"})
	}))
	defer api.Close()
	c := NewControllerWithPool(p)
	c.Configure(testConfig(api.URL))
	defer c.Stop()
	for i := 0; i < 19; i++ {
		runSiteOutcome(c, context.Background(), c.Config(), c.rev, url, false)
	}
	done := make(chan struct{})
	go func() { defer close(done); runSiteOutcome(c, context.Background(), c.Config(), c.rev, url, false) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("review did not reach retest")
	}
	if _, err := p.Record(url, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done
	if active, _ := p.Counts(); active != 3 {
		t.Fatal("other instance success overwritten", active)
	}
	if s := p.states[url]; s.Failures != 0 || !s.QuarantinedAt.IsZero() {
		t.Fatal("success streak resurrected", s)
	}
}

func TestSiteChecksSuccessRecordedBeforeSlowRecovery(t *testing.T) {
	url := "https://a.example/"
	p, err := OpenSitePool([]string{url}, "")
	if err != nil {
		t.Fatal(err)
	}
	token := failSite(t, p, url, time.Now())
	entered, release := make(chan struct{}), make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxies/US/delay" {
			close(entered)
			<-release
			w.Write([]byte(`{"delay":100}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/delay") {
			w.Write([]byte(`{"delay":100}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": []string{"US", "Japan"}, "now": "Japan"})
	}))
	defer api.Close()
	c := NewControllerWithPool(p)
	cfg := testConfig(api.URL)
	cfg.RecoveryEnabled = true
	cfg.RecoveryInitialDelaySeconds = 0
	c.Configure(cfg)
	defer c.Stop()
	c.mu.Lock()
	c.recovery.startedAt = time.Now().Add(-time.Hour)
	c.recovery.nextCheck = time.Now().Add(-time.Hour)
	c.mu.Unlock()
	done := make(chan struct{})
	go func() { defer close(done); c.Check(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not start")
	}
	applied, saveErr := p.FinishReview(url, token, true, time.Now())
	close(release)
	<-done
	if applied || saveErr != nil {
		t.Fatal("normal success deferred past recovery; old review quarantined site", applied, saveErr)
	}
}

func TestSiteChecksCancelledRetryReleasesLease(t *testing.T) {
	url := "https://a.example/"
	p, err := OpenSitePool([]string{url}, "")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	token := failSite(t, p, url, past)
	if _, err = p.FinishReview(url, token, true, past); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer api.Close()
	c := NewControllerWithPool(p)
	cfg := testConfig(api.URL)
	c.Configure(cfg)
	defer c.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.retryQuarantinedSite(ctx, cfg, c.rev, gateway.NewMihomo(cfg)) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("retry did not start")
	}
	cancel()
	<-done
	if next, _ := p.ClaimRetry(time.Now().Add(siteRetrySpacing + time.Second)); next != url {
		t.Fatal("cancelled retry kept lease beyond global retry spacing")
	}
}

type cancelAfterSiteClaim struct {
	context.Context
	calls int
}

func (ctx *cancelAfterSiteClaim) Err() error {
	ctx.calls++
	if ctx.calls >= 2 {
		return context.Canceled
	}
	return nil
}

func TestSiteChecksCancelAtClaimReturnReleasesLease(t *testing.T) {
	url := "https://a.example/"
	p, err := OpenSitePool([]string{url}, "")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	token := failSite(t, p, url, past)
	if _, err = p.FinishReview(url, token, true, past); err != nil {
		t.Fatal(err)
	}
	c := NewControllerWithPool(p)
	cfg := testConfig("http://127.0.0.1:1")
	c.Configure(cfg)
	defer c.Stop()
	ctx := &cancelAfterSiteClaim{Context: context.Background()}
	c.retryQuarantinedSite(ctx, cfg, c.rev, gateway.NewMihomo(cfg))
	if site, _ := p.ClaimRetry(time.Now().Add(siteRetrySpacing + time.Second)); site != url {
		t.Fatal("claim-time cancellation kept retry lease")
	}
}

func TestSiteChecksReviewSuccessAndControlError(t *testing.T) {
	for _, code := range []int{200, 401} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			codes := map[string]int{"https://b.example/": code, "https://c.example/": code}
			api, _ := siteCheckAPI(t, codes)
			p := testSitePool(t, "")
			c := NewControllerWithPool(p)
			c.Configure(testConfig(api.URL))
			defer c.Stop()
			for i := 0; i < 20; i++ {
				runSiteOutcome(c, context.Background(), c.Config(), c.rev, "https://a.example/", false)
			}
			if active, _ := p.Counts(); active != 3 {
				t.Fatal("review quarantined a reachable/uncertain site")
			}
			failures := p.states["https://a.example/"].Failures
			if code == 200 && failures != 0 {
				t.Fatal("successful review did not reset")
			}
			if code == 401 && failures != 20 {
				t.Fatal("control error changed streak")
			}
		})
	}
}
