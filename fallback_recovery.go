package main

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"time"
)

const countryHealthTimeoutSeconds = 5

// Returns true when this cycle recovered, was cancelled, or must not continue
// with the old selection. False resumes the existing main-group failover.
func (c *Controller) recoverCurrentGroup(ctx context.Context, rev uint64, client *Mihomo, group ProxyGroup, site string, guard *ProxyGroup) bool {
	if ctx.Err() != nil {
		return true
	}
	info, err := client.GroupInfo(ctx, group.Now)
	if err != nil {
		c.fail(rev, err)
		return true
	}
	if info.Type != "Fallback" && info.Type != "Selector" && info.Type != "URLTest" {
		return false
	}
	// Keep this baseline through the subsequent main-group delay and selection.
	*guard = info
	if info.Type != "Selector" && info.Fixed != "" {
		c.update(rev, func(s *Status) {
			addEvent(s, "info", "当前自动分组存在手动固定，不解除固定，继续主分组流程："+info.Name)
		})
		return false
	}
	healthSite, expected := info.TestURL, info.ExpectedStatus
	if info.Type == "Selector" {
		if expected == "" {
			expected = "200-299"
		}
		if healthSite == "" {
			choices := c.pool.Pick(site, 1)
			if len(choices) == 0 {
				return false
			}
			healthSite = choices[0]
		}
	}
	healthURL, parseErr := url.Parse(healthSite)
	if parseErr != nil || healthURL.Host == "" || (healthURL.Scheme != "http" && healthURL.Scheme != "https") || len(info.All) == 0 {
		c.update(rev, func(s *Status) {
			addEvent(s, "warning", "当前分组缺少有效健康检查地址或成员，继续主分组探查："+group.Now)
		})
		return false
	}
	if _, ok := c.checkedInnerGroup(ctx, rev, client, group, info); !ok {
		return true
	}
	if !c.update(rev, func(s *Status) {
		s.Phase = "testing"
		addEvent(s, "info", "达到失败阈值，先按当前分组健康检查配置探查直接成员："+group.Now)
	}) {
		return true
	}
	healthConfig := client.cfg
	healthConfig.TimeoutSeconds = countryHealthTimeoutSeconds
	healthClient := NewMihomo(healthConfig)
	if info.Type == "Fallback" {
		_, err = healthClient.GroupDelays(ctx, group.Now, healthSite, expected)
	} else {
		var delays map[string]int
		delays, err = healthClient.MemberDelays(ctx, info.All, healthSite, expected)
		if err == nil {
			if info.Type == "Selector" {
				var proceed bool
				info, proceed = c.repairSelector(ctx, rev, client, group, info, delays)
				*guard = info
				if !proceed {
					return true
				}
			} else if !c.awaitURLTestChoice(ctx, rev, client, group, info, delays) {
				return true
			}
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return true
		}
		var apiErr *APIError
		// Mihomo reports 504 when all direct members time out. Still recheck A.
		if info.Type != "Fallback" || !errors.As(err, &apiErr) || apiErr.Code != 504 {
			c.fail(rev, err)
			return true
		}
	}
	if _, ok := c.checkedInnerGroup(ctx, rev, client, group, info); !ok {
		return true
	}
	choices := c.pool.Pick(site, 1)
	if len(choices) == 0 {
		c.update(rev, func(s *Status) {
			addEvent(s, "warning", "无其他有效站点可用于复验，继续主分组探查："+group.Now)
		})
		return false
	}
	recheckSite := choices[0]
	delay, err := client.ProbeGroup(ctx, group.Now, recheckSite)
	if ctx.Err() != nil {
		return true
	}
	if err != nil && !isNodeFailure(err) {
		c.fail(rev, err)
		return true
	}
	if _, ok := c.checkedInnerGroup(ctx, rev, client, group, info); !ok {
		return true
	}
	c.recordGroupProbe(rev, group.All, group.Now, delay, err)
	if err != nil {
		c.update(rev, func(s *Status) {
			addEvent(s, "warning", "当前分组出口复验仍失败，继续主分组探查："+group.Now+"；站点："+recheckSite)
		})
		return false
	}
	if c.update(rev, func(s *Status) {
		c.allUnavailable = false
		s.Failures = 0
		s.Delay = delay
		s.Phase = "healthy"
		now := time.Now()
		s.LastSite = recheckSite
		s.LastCheck = &now
		addEvent(s, "success", "当前分组直接成员探查后出口复验成功，保留分组："+group.Now+"；站点："+recheckSite)
	}) {
		c.recordSiteOutcome(ctx, rev, recheckSite, true)
	}
	return true
}

func (c *Controller) sameFallbackSelection(ctx context.Context, rev uint64, client *Mihomo, original ProxyGroup) bool {
	if ctx.Err() != nil {
		return false
	}
	latest, err := client.Group(ctx)
	if err != nil {
		c.fail(rev, err)
		return false
	}
	if c.failureSelectionChanged(rev, original, latest) {
		return false
	}
	return c.update(rev, func(*Status) {}) && ctx.Err() == nil
}

func (c *Controller) failureSelectionChanged(rev uint64, original, latest ProxyGroup) bool {
	if latest.Now != original.Now || !slices.Equal(latest.All, original.All) {
		c.observeRecoveryGroup(rev, latest)
		c.update(rev, func(s *Status) {
			s.Current = latest.Now
			s.Phase = "waiting"
			addEvent(s, "info", "主分组选择或成员顺序已变化，跳过本轮故障处理")
		})
		return true
	}
	return false
}
