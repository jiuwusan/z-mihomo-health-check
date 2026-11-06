package service

import (
	"errors"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/repository"
)

const (
	siteFailureThreshold = 20
	siteReviewCooldown   = 30 * time.Minute
	siteRetryInterval    = time.Hour
	siteRetrySpacing     = time.Minute
	siteRetryLease       = 2 * time.Minute
)

type siteHealth = domain.SiteHealth

type siteLease struct {
	token uint64
	until time.Time
}

// One shared pool per service process. Never calls back into a controller.
type SitePool struct {
	mu         sync.Mutex
	sites      []string
	known      map[string]bool
	states     map[string]siteHealth
	generation map[string]uint64
	leases     map[string]siteLease
	sequence   uint64
	nextRetry  time.Time
	filename   string
}

func OpenSitePool(sites []string, filename string) (*SitePool, error) {
	p := &SitePool{known: map[string]bool{}, states: map[string]siteHealth{}, generation: map[string]uint64{}, leases: map[string]siteLease{}, filename: filename}
	for _, s := range sites {
		if !p.known[s] {
			p.sites = append(p.sites, s)
			p.known[s] = true
		}
	}
	if filename == "" {
		return p, nil
	}
	saved, err := repository.LoadSitePool(filename)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return p, nil
	}
	for url, s := range saved.Sites {
		if s.Failures < 0 || s.Failures > siteFailureThreshold || (!s.QuarantinedAt.IsZero() && (s.Failures != siteFailureThreshold || s.RetryAt.IsZero())) || (s.QuarantinedAt.IsZero() && !s.RetryAt.IsZero()) {
			return nil, errors.New("全局站点池状态包含无效记录")
		}
		if p.known[url] {
			p.states[url] = s
		}
	}
	return p, nil
}

func (p *SitePool) Counts() (active, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	total = len(p.sites)
	active = total
	for _, s := range p.states {
		if !s.QuarantinedAt.IsZero() {
			active--
		}
	}
	return
}
func (p *SitePool) Pick(exclude string, limit int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := []string{}
	for _, s := range p.sites {
		if s != exclude && p.states[s].QuarantinedAt.IsZero() {
			items = append(items, s)
		}
	}
	rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return items[:min(max(limit, 0), len(items))]
}
func (p *SitePool) save(next map[string]siteHealth) error {
	if p.filename != "" {
		if repository.SaveSitePool(p.filename, next) != nil {
			return errors.New("全局站点池状态保存失败，状态暂未变更")
		}
	}
	p.states = next
	return nil
}
func (p *SitePool) changed(url string, state *siteHealth) error {
	next := make(map[string]siteHealth, len(p.states)+1)
	for k, v := range p.states {
		next[k] = v
	}
	if state == nil {
		delete(next, url)
	} else {
		next[url] = *state
	}
	if err := p.save(next); err != nil {
		return err
	}
	return nil
}

func (p *SitePool) advance(url string) uint64 {
	p.sequence++
	p.generation[url] = p.sequence
	return p.sequence
}

func clearedSite(s siteHealth, now time.Time) *siteHealth {
	if now.Before(s.ReviewAfter) {
		return &siteHealth{ReviewAfter: s.ReviewAfter}
	}
	return nil
}

// Only normal exit probes call Record. A nonzero token reserves one review.
func (p *SitePool) Record(url string, success bool, now time.Time) (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, exists := p.states[url]
	if !p.known[url] || !s.QuarantinedAt.IsZero() {
		return 0, nil
	}
	if success {
		if !exists {
			return 0, nil
		}
		if err := p.changed(url, clearedSite(s, now)); err != nil {
			return 0, err
		}
		p.advance(url)
		return 0, nil
	}
	s.Failures = min(s.Failures+1, siteFailureThreshold)
	due := s.Failures == siteFailureThreshold && !now.Before(s.ReviewAfter)
	if due {
		s.ReviewAfter = now.Add(siteReviewCooldown)
	}
	if err := p.changed(url, &s); err != nil {
		return 0, err
	}
	if due {
		return p.advance(url), nil
	}
	return 0, nil
}
func (p *SitePool) ReviewCurrent(url string, token uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.states[url]
	return ok && token != 0 && p.generation[url] == token && s.QuarantinedAt.IsZero()
}

// false quarantine means the explicit recheck succeeded, not an uncertain exit.
func (p *SitePool) FinishReview(url string, token uint64, quarantine bool, now time.Time) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.states[url]
	if !ok || token == 0 || p.generation[url] != token || !s.QuarantinedAt.IsZero() {
		return false, nil
	}
	next := clearedSite(s, now)
	if quarantine {
		s.QuarantinedAt = now
		s.RetryAt = now.Add(siteRetryInterval)
		next = &s
	}
	if err := p.changed(url, next); err != nil {
		return false, err
	}
	p.advance(url)
	return true, nil
}

// A global rate limit and expiring lease bound retry traffic across instances.
func (p *SitePool) ClaimRetry(now time.Time) (string, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if now.Before(p.nextRetry) {
		return "", 0
	}
	items := []string{}
	for url, s := range p.states {
		if !s.QuarantinedAt.IsZero() && !now.Before(s.RetryAt) && !now.Before(p.leases[url].until) {
			items = append(items, url)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := p.states[items[i]].RetryAt, p.states[items[j]].RetryAt
		if a.Equal(b) {
			return items[i] < items[j]
		}
		return a.Before(b)
	})
	if len(items) == 0 {
		return "", 0
	}
	url := items[0]
	token := p.advance(url)
	p.leases[url] = siteLease{token: token, until: now.Add(siteRetryLease)}
	p.nextRetry = now.Add(siteRetrySpacing)
	return url, token
}

// Cancellation releases only the transient lease, never commits an old result.
func (p *SitePool) AbandonRetry(url string, token uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if token != 0 && p.leases[url].token == token {
		delete(p.leases, url)
	}
}
func (p *SitePool) FinishRetry(url string, token uint64, success, nodeFailure bool, now time.Time) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.states[url]
	if !ok || token == 0 || p.leases[url].token != token || p.generation[url] != token || s.QuarantinedAt.IsZero() {
		return false, nil
	}
	var next *siteHealth
	if !success {
		s.RetryAt = now.Add(siteRetrySpacing)
		if nodeFailure {
			s.RetryAt = now.Add(siteRetryInterval)
		}
		next = &s
	}
	if err := p.changed(url, next); err != nil {
		return false, err
	}
	p.advance(url)
	delete(p.leases, url)
	return success, nil
}
