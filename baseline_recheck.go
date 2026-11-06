package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

const defaultBaselineURL = "http://www.gstatic.com/generate_204"
const baselineRecheckAttempts = 3

// True means the exit recovered or this cycle must stop. Only three confirmed
// node failures return false and allow the existing recovery flow to proceed.
func (c *Controller) recheckBaselineExit(ctx context.Context, rev uint64, client *Mihomo, group ProxyGroup) bool {
	if ctx.Err() != nil {
		return true
	}
	info, err := client.GroupInfo(ctx, group.Now)
	if err != nil {
		c.fail(rev, err)
		return true
	}
	site, expected := info.TestURL, info.ExpectedStatus
	if site == "" {
		site, expected = defaultBaselineURL, "204"
	} else if info.Type == "Selector" && expected == "" {
		expected = "200-299"
	}
	u, err := url.Parse(site)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		c.fail(rev, errors.New("当前分组基准复验地址无效，请检查订阅健康检查 URL"))
		return true
	}
	for attempt := 1; attempt <= baselineRecheckAttempts; attempt++ {
		if _, ok := c.checkedInnerGroup(ctx, rev, client, group, info); !ok {
			return true
		}
		if !c.update(rev, func(s *Status) {
			now := time.Now()
			s.Phase = "checking"
			s.LastSite, s.LastCheck, s.Delay = site, &now, 0
			addEvent(s, "info", fmt.Sprintf("达到失败阈值，正在执行基准出口复验（%d/%d）；分组：%s；站点：%s", attempt, baselineRecheckAttempts, group.Now, site))
		}) {
			return true
		}
		delay, err := client.ProbeGroupExpected(ctx, group.Now, site, expected)
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
		if err == nil {
			c.update(rev, func(s *Status) {
				c.allUnavailable = false
				s.Failures, s.Delay, s.Phase = 0, delay, "healthy"
				addEvent(s, "success", fmt.Sprintf("基准出口复验成功（%d/%d），保留当前分组并清零出口失败计数：%s；站点：%s", attempt, baselineRecheckAttempts, group.Now, site))
			})
			return true
		}
		if !c.update(rev, func(s *Status) {
			s.Phase = "warning"
			addEvent(s, "warning", fmt.Sprintf("基准出口复验失败（%d/%d）：%s；分组：%s；站点：%s", attempt, baselineRecheckAttempts, err, group.Now, site))
		}) {
			return true
		}
	}
	return !c.update(rev, func(s *Status) {
		addEvent(s, "info", "基准出口连续 3 次复验失败，继续组内自恢复及主分组故障处理："+group.Now)
	})
}
