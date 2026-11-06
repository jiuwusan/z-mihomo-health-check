package service

import (
	"context"
	"errors"
	"time"

	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/gateway"
)

func (c *Controller) SyncNodeInfo(parent context.Context) (domain.Status, error) {
	c.mu.Lock()
	cfg, rev, base, currentVersion := c.cfg, c.rev, c.ctx, c.currentVersion
	c.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(base, cancel)
	defer stop()
	started := time.Now()
	current, results, err := gateway.NewMihomo(cfg).NodeInfo(ctx)
	if err != nil {
		return domain.Status{}, err
	}
	if ctx.Err() != nil {
		return domain.Status{}, ctx.Err()
	}
	if !c.update(rev, func(s *domain.Status) {
		previous := map[string]domain.GroupResult{}
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
		return domain.Status{}, errors.New("实例配置已变化，请重新刷新节点信息")
	}
	return c.Status(), nil
}
