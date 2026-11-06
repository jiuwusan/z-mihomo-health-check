package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testSitePool(t *testing.T, filename string) *SitePool {
	t.Helper()
	p, err := OpenSitePool([]string{"https://a.example/", "https://b.example/", "https://c.example/"}, filename)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func failSite(t *testing.T, p *SitePool, site string, now time.Time) uint64 {
	t.Helper()
	var token uint64
	for i := 0; i < 20; i++ {
		v, err := p.Record(site, false, now)
		if err != nil {
			t.Fatal(err)
		}
		if i < 19 && v != 0 {
			t.Fatal("review before threshold")
		}
		token = v
	}
	if token == 0 {
		t.Fatal("missing threshold review")
	}
	return token
}

func TestSitePoolThresholdResetCooldownAndStaleReview(t *testing.T) {
	p := testSitePool(t, "")
	now := time.Now()
	url := "https://a.example/"
	for i := 0; i < 19; i++ {
		if token, err := p.Record(url, false, now); token != 0 || err != nil {
			t.Fatal(token, err)
		}
	}
	if _, err := p.Record(url, true, now); err != nil {
		t.Fatal(err)
	}
	token := failSite(t, p, url, now)
	if v, _ := p.Record(url, false, now.Add(time.Minute)); v != 0 {
		t.Fatal("review during cooldown")
	}
	if !p.ReviewCurrent(url, token) {
		t.Fatal("same-direction failure invalidated pending review")
	}
	if _, err := p.Record(url, true, now); err != nil {
		t.Fatal(err)
	}
	if ok, _ := p.FinishReview(url, token, true, now); ok {
		t.Fatal("success did not invalidate old review")
	}
	now = now.Add(siteReviewCooldown + time.Second)
	token = failSite(t, p, url, now)
	if ok, err := p.FinishReview(url, token, true, now); !ok || err != nil {
		t.Fatal(ok, err)
	}
	for i := 0; i < 10; i++ {
		for _, s := range p.Pick("", 3) {
			if s == url {
				t.Fatal("isolated site sampled")
			}
		}
	}
	if _, err := p.Record(url, true, now); err != nil {
		t.Fatal(err)
	}
	if active, total := p.Counts(); active != 2 || total != 3 {
		t.Fatal(active, total)
	}
}

func TestSitePoolReviewCooldownSurvivesSuccess(t *testing.T) {
	for _, normalSuccess := range []bool{false, true} {
		t.Run(map[bool]string{false: "review_success", true: "normal_success"}[normalSuccess], func(t *testing.T) {
			p := testSitePool(t, "")
			now := time.Now()
			url := "https://a.example/"
			token := failSite(t, p, url, now)
			if normalSuccess {
				if _, err := p.Record(url, true, now); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := p.FinishReview(url, token, false, now); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 20; i++ {
				token, err := p.Record(url, false, now)
				if err != nil || token != 0 {
					t.Fatal("success bypassed 30-minute cooldown", token, err)
				}
			}
		})
	}
}

func TestSitePoolPersistenceAndRecoveryThrottle(t *testing.T) {
	file := filepath.Join(t.TempDir(), "site-pool.json")
	p := testSitePool(t, file)
	now := time.Now()
	for _, url := range []string{"https://a.example/", "https://b.example/"} {
		token := failSite(t, p, url, now)
		if _, err := p.FinishReview(url, token, true, now); err != nil {
			t.Fatal(err)
		}
	}
	p = testSitePool(t, file)
	if active, _ := p.Counts(); active != 1 {
		t.Fatal(active)
	}
	if url, _ := p.ClaimRetry(now.Add(59 * time.Minute)); url != "" {
		t.Fatal("early retry")
	}
	url, token := p.ClaimRetry(now.Add(time.Hour))
	if url == "" || token == 0 {
		t.Fatal("missing due retry")
	}
	if next, _ := p.ClaimRetry(now.Add(time.Hour)); next != "" {
		t.Fatal("global retry throttle bypassed")
	}
	if restored, err := p.FinishRetry(url, token, true, false, now.Add(time.Hour)); !restored || err != nil {
		t.Fatal(restored, err)
	}
	p = testSitePool(t, file)
	if active, _ := p.Counts(); active != 2 {
		t.Fatal("restore not persisted", active)
	}
}

func TestSitePoolWriteFailureDoesNotPublishAndCorruptionFails(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "site-pool.json")
	p := testSitePool(t, file)
	if err := os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Record("https://a.example/", false, time.Now()); err == nil {
		t.Fatal("write failure hidden")
	}
	if len(p.states) != 0 {
		t.Fatal("failed write published")
	}
	bad := filepath.Join(root, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"sites":{"https://a.example/":{"failures":99}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSitePool([]string{"https://a.example/"}, bad); err == nil {
		t.Fatal("invalid state accepted")
	}
}

func TestSitePoolSharedConcurrentThreshold(t *testing.T) {
	p := testSitePool(t, "")
	var wg sync.WaitGroup
	var mu sync.Mutex
	reviews := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := p.Record("https://a.example/", false, time.Now())
			if err != nil {
				t.Error(err)
			}
			if token != 0 {
				mu.Lock()
				reviews++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if reviews != 1 {
		t.Fatal("duplicate global reviews", reviews)
	}
}

func TestSitePoolPendingCountsSurviveRestartAndRetryBackoff(t *testing.T) {
	file := filepath.Join(t.TempDir(), "site-pool.json")
	p := testSitePool(t, file)
	now := time.Now()
	url := "https://a.example/"
	for i := 0; i < 19; i++ {
		if _, err := p.Record(url, false, now); err != nil {
			t.Fatal(err)
		}
	}
	p = testSitePool(t, file)
	token, err := p.Record(url, false, now)
	if err != nil || token == 0 {
		t.Fatal("pending streak lost", token, err)
	}
	if _, err = p.FinishReview(url, token, true, now); err != nil {
		t.Fatal(err)
	}
	retryURL, retryToken := p.ClaimRetry(now.Add(time.Hour))
	if retryURL != url {
		t.Fatal(retryURL)
	}
	if restored, err := p.FinishRetry(url, retryToken, false, true, now.Add(time.Hour)); restored || err != nil {
		t.Fatal(restored, err)
	}
	if site, _ := p.ClaimRetry(now.Add(time.Hour + 10*time.Minute)); site != "" {
		t.Fatal("failed exit retried before one hour")
	}
	_, oldToken := p.ClaimRetry(now.Add(2 * time.Hour))
	_, newToken := p.ClaimRetry(now.Add(2*time.Hour + siteRetryLease))
	if newToken == 0 || oldToken == newToken {
		t.Fatal("expired retry lease not recovered")
	}
	if restored, err := p.FinishRetry(url, oldToken, true, false, now.Add(2*time.Hour)); restored || err != nil {
		t.Fatal("stale lease restored site", restored, err)
	}
	if restored, err := p.FinishRetry(url, newToken, true, false, now.Add(2*time.Hour+siteRetryLease)); !restored || err != nil {
		t.Fatal(restored, err)
	}
}
