package gateway

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"
)

// Known failures have value 0; missing entries remain unknown, not failed.
func (m *Mihomo) MemberDelays(parent context.Context, members []string, site, expected string) (map[string]int, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	results := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	index := 0
	var firstErr error
	for i := 0; i < min(8, len(members)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if index >= len(members) || ctx.Err() != nil {
					mu.Unlock()
					return
				}
				name := members[index]
				index++
				mu.Unlock()
				delay, known, err := m.memberHealth(ctx, name, site, expected)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
						cancel()
					}
				} else if known {
					results[name] = delay
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if parent.Err() != nil {
		return nil, parent.Err()
	}
	return results, nil
}

func (m *Mihomo) memberHealth(ctx context.Context, name, site, expected string) (int, bool, error) {
	_, err := m.ProbeGroupExpected(ctx, name, site, expected)
	if err != nil {
		if isNodeFailure(err) {
			return 0, true, nil
		}
		return 0, false, err
	}
	var data struct {
		Extra map[string]struct {
			Alive   *bool `json:"alive"`
			History []struct {
				Time  time.Time `json:"time"`
				Delay int       `json:"delay"`
			} `json:"history"`
		} `json:"extra"`
	}
	if err = m.requestLimited(ctx, "GET", "/proxies/"+url.PathEscape(name), nil, nil, &data, 8*1024*1024); err != nil {
		return 0, false, err
	}
	health, ok := data.Extra[site]
	if !ok {
		return 0, false, nil
	}
	var latest time.Time
	delay := 0
	for _, h := range health.History {
		if !h.Time.IsZero() && h.Delay >= 0 && h.Delay <= 65535 && (latest.IsZero() || h.Time.After(latest)) {
			latest = h.Time
			delay = h.Delay
		}
	}
	if latest.IsZero() {
		return 0, false, nil
	}
	if health.Alive != nil && !*health.Alive {
		delay = 0
	}
	return delay, true, nil
}

func isNodeFailure(err error) bool { var e *APIError; return errors.As(err, &e) && e.Node }
