package service

import (
	"context"
	"slices"
	"time"

	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/gateway"
)

const urlTestRoutingCacheWait = 10 * time.Second

func (c *Controller) checkedInnerGroup(ctx context.Context, rev uint64, client *gateway.Mihomo, main, expected gateway.ProxyGroup) (gateway.ProxyGroup, bool) {
	if !c.sameFallbackSelection(ctx, rev, client, main) {
		return gateway.ProxyGroup{}, false
	}
	actual, err := client.GroupInfo(ctx, expected.Name)
	if err != nil {
		c.fail(rev, err)
		return gateway.ProxyGroup{}, false
	}
	changed := actual.Type != expected.Type || !slices.Equal(actual.All, expected.All) || actual.TestURL != expected.TestURL || actual.ExpectedStatus != expected.ExpectedStatus || actual.Fixed != expected.Fixed || (actual.Type == "Selector" && actual.Now != expected.Now)
	if changed {
		c.update(rev, func(s *domain.Status) {
			s.Phase = "waiting"
			addEvent(s, "info", "国家分组选择、固定或配置已变化，停止旧组内恢复流程："+expected.Name)
		})
		return actual, false
	}
	return actual, c.update(rev, func(*domain.Status) {}) && ctx.Err() == nil
}

func (c *Controller) repairSelector(ctx context.Context, rev uint64, client *gateway.Mihomo, main, info gateway.ProxyGroup, delays map[string]int) (gateway.ProxyGroup, bool) {
	if _, ok := c.checkedInnerGroup(ctx, rev, client, main, info); !ok {
		return info, false
	}
	currentDelay, known := delays[info.Now]
	if !known || currentDelay > 0 {
		return info, true
	}
	member := ""
	for _, name := range info.All {
		if delays[name] > 0 {
			member = name
			break
		}
	}
	if member == "" {
		return info, true
	}
	if _, ok := c.checkedInnerGroup(ctx, rev, client, main, info); !ok {
		return info, false
	}
	err := client.SelectGroup(ctx, info.Name, member)
	if ctx.Err() != nil {
		return info, false
	}
	if !c.sameFallbackSelection(ctx, rev, client, main) {
		return info, false
	}
	expected := info
	expected.Now = member
	actual, ok := c.checkedInnerGroup(ctx, rev, client, main, expected)
	if !ok {
		if err != nil {
			c.fail(rev, err)
		}
		return info, false
	}
	c.update(rev, func(s *domain.Status) {
		for i := range s.Results {
			if s.Results[i].Name == info.Name {
				s.Results[i].Node = ""
			}
		}
		addEvent(s, "info", "select 当前成员不可用，已按原始顺序组内切换："+info.Name+"；"+info.Now+" → "+member)
	})
	return actual, true
}

func (c *Controller) awaitURLTestChoice(ctx context.Context, rev uint64, client *gateway.Mihomo, main, info gateway.ProxyGroup, delays map[string]int) bool {
	actual, ok := c.checkedInnerGroup(ctx, rev, client, main, info)
	if !ok {
		return false
	}
	healthy := false
	for _, delay := range delays {
		if delay > 0 {
			healthy = true
			break
		}
	}
	if !healthy || delays[actual.Now] > 0 {
		return true
	}
	if !c.update(rev, func(s *domain.Status) {
		s.Phase = "testing"
		addEvent(s, "info", "等待 url-test 选路缓存刷新，由 Mihomo 自动选择："+info.Name)
	}) {
		return false
	}
	timer := time.NewTimer(urlTestRoutingCacheWait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
	}
	_, ok = c.checkedInnerGroup(ctx, rev, client, main, info)
	return ok
}
