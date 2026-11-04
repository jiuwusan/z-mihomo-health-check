package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func isNodeFailure(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Node
}

// Shared pool mutations are guarded by the same revision lock as public status.
// Pool persistence never calls back into controllers, keeping lock order fixed.
func (c *Controller) siteUpdate(ctx context.Context, rev uint64, fn func(*Status)) bool {
	return c.update(rev, func(s *Status) {
		if ctx.Err() == nil {
			fn(s)
		}
	}) && ctx.Err() == nil
}

func (c *Controller) recordSiteOutcome(ctx context.Context, rev uint64, site string, success bool) uint64 {
	var token uint64
	if !c.siteUpdate(ctx, rev, func(s *Status) {
		var err error
		token, err = c.pool.Record(site, success, time.Now())
		if err != nil {
			addEvent(s, "error", err.Error())
		}
	}) {
		return 0
	}
	return token
}

func (c *Controller) reviewSite(ctx context.Context, cfg Config, rev uint64, site string, token uint64) {
	client := NewMihomo(cfg)
	current := func() bool { return ctx.Err() == nil && c.pool.ReviewCurrent(site, token) }
	controls := c.pool.Pick(site, 2)
	c.siteUpdate(ctx, rev, func(s *Status) {
		addEvent(s, "info", "站点连续失败达到 20 次，开始复核；站点："+site)
	})
	defer c.siteUpdate(ctx, rev, func(s *Status) {
		if current() {
			addEvent(s, "warning", "站点复核未确认独立故障，保留站点并冷却 30 分钟；站点："+site)
		}
	})
	if len(controls) == 0 || !current() {
		return
	}
	before, err := client.Group(ctx)
	if err != nil {
		return
	}
	for _, url := range controls {
		if !current() {
			return
		}
		if _, err = client.Probe(ctx, url); err != nil {
			return
		}
	}
	if !current() {
		return
	}
	_, err = client.Probe(ctx, site)
	if err != nil && !isNodeFailure(err) {
		return
	}
	quarantine := err != nil
	if !current() {
		return
	}
	after, groupErr := client.Group(ctx)
	if groupErr != nil || before.Now != after.Now {
		return
	}
	c.siteUpdate(ctx, rev, func(s *Status) {
		applied, saveErr := c.pool.FinishReview(site, token, quarantine, time.Now())
		if saveErr != nil {
			addEvent(s, "error", saveErr.Error())
			return
		}
		if !applied {
			return
		}
		if quarantine {
			addEvent(s, "warning", "站点复核失败，已全局临时隔离，1 小时后复测；站点："+site)
		} else {
			addEvent(s, "success", "站点复核成功，已清零连续失败计数；站点："+site)
		}
	})
}

func (c *Controller) retryQuarantinedSite(ctx context.Context, cfg Config, rev uint64, client *Mihomo) {
	var site string
	var token uint64
	claimed := c.siteUpdate(ctx, rev, func(s *Status) { site, token = c.pool.ClaimRetry(time.Now()) })
	defer c.pool.AbandonRetry(site, token)
	if !claimed || site == "" {
		return
	}
	_, err := client.Probe(ctx, site)
	c.siteUpdate(ctx, rev, func(s *Status) {
		restored, saveErr := c.pool.FinishRetry(site, token, err == nil, isNodeFailure(err), time.Now())
		if saveErr != nil {
			addEvent(s, "error", saveErr.Error())
			return
		}
		if restored {
			addEvent(s, "success", "隔离站点复测成功，已恢复全局抽取；站点："+site)
		} else if err != nil {
			addEvent(s, "warning", fmt.Sprintf("隔离站点复测未恢复：%s；站点：%s", err, site))
		}
	})
}
