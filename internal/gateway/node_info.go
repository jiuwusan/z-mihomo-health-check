package gateway

import (
	"context"
	"errors"
	"time"

	"z-mihomo-health-check/internal/domain"
)

type proxyMetadata struct {
	Type    string   `json:"type"`
	Now     string   `json:"now"`
	All     []string `json:"all"`
	Alive   *bool    `json:"alive"`
	History []struct {
		Time  time.Time `json:"time"`
		Delay int       `json:"delay"`
	} `json:"history"`
}

func (m *Mihomo) NodeInfo(ctx context.Context) (string, []domain.GroupResult, error) {
	var data struct {
		Proxies map[string]proxyMetadata `json:"proxies"`
	}
	// /proxies includes per-URL history, so allow a larger bounded response.
	if err := m.requestLimited(ctx, "GET", "/proxies", nil, nil, &data, 8*1024*1024); err != nil {
		return "", nil, err
	}
	root, ok := data.Proxies[m.cfg.Group]
	if !ok || root.Type != "Selector" || len(root.All) == 0 {
		return "", nil, errors.New("目标必须是包含成员的 select 分组")
	}
	results := make([]domain.GroupResult, 0, len(root.All))
	for _, name := range root.All {
		r := domain.GroupResult{Name: name, Source: "mihomo"}
		seen := map[string]bool{}
		current := name
		for current != "" && !seen[current] {
			seen[current] = true
			p, exists := data.Proxies[current]
			if !exists {
				break
			}
			if p.Now != "" {
				current = p.Now
				continue
			}
			if len(p.All) > 0 {
				break
			}
			r.Node = current
			for _, h := range p.History {
				if h.Time.IsZero() || h.Delay < 0 || h.Delay > 65535 {
					continue
				}
				if r.CheckedAt == nil || h.Time.After(*r.CheckedAt) {
					at := h.Time
					r.CheckedAt = &at
					r.Delay = h.Delay
					r.StatusKnown = true
					r.Available = h.Delay > 0
				}
			}
			if p.Alive != nil && r.CheckedAt != nil {
				r.Available = *p.Alive
			}
			if !r.Available {
				r.Delay = 0
			}
			break
		}
		results = append(results, r)
	}
	return root.Now, results, nil
}
