package main

import (
	"context"
	"errors"
	"time"
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

func (m *Mihomo) nodeInfo(ctx context.Context) (string, []GroupResult, error) {
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
	results := make([]GroupResult, 0, len(root.All))
	for _, name := range root.All {
		r := GroupResult{Name: name, Source: "mihomo"}
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

func (c *Controller) SyncNodeInfo(parent context.Context) (Status, error) {
	c.mu.Lock()
	cfg, rev, base, currentVersion := c.cfg, c.rev, c.ctx, c.currentVersion
	c.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(base, cancel)
	defer stop()
	started := time.Now()
	current, results, err := NewMihomo(cfg).nodeInfo(ctx)
	if err != nil {
		return Status{}, err
	}
	if ctx.Err() != nil {
		return Status{}, ctx.Err()
	}
	if !c.update(rev, func(s *Status) {
		previous := map[string]GroupResult{}
		for _, r := range s.Results {
			previous[r.Name] = r
		}
		for i, r := range results {
			if old, ok := previous[r.Name]; ok && old.Source == "" && old.CheckedAt != nil && !old.CheckedAt.Before(started) {
				old.Node = r.Node
				results[i] = old
			}
		}
		now := time.Now()
		s.NodeInfoSyncedAt = &now
		s.Results = results
		if c.currentVersion == currentVersion {
			s.Current = current
		}
	}) {
		return Status{}, errors.New("实例配置已变化，请重新刷新节点信息")
	}
	return c.Status(), nil
}
