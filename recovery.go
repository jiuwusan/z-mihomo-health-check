package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

type RecoveryStatus struct {
	Enabled            bool       `json:"enabled"`
	Candidate          string     `json:"candidate"`
	Successes          int        `json:"successes"`
	Threshold          int        `json:"threshold"`
	InitialWaitSeconds int        `json:"initial_wait_seconds"`
	LastCheck          *time.Time `json:"last_check"`
	Message            string     `json:"message"`
}

type recoveryState struct {
	startedAt time.Time
	checked   bool
	nextCheck time.Time
	current   string
	order     []string
}

// Called only while holding c.mu.
func (c *Controller) restartRecoveryWait(now time.Time) {
	c.recovery.startedAt = now
	c.recovery.checked = false
	c.recovery.nextCheck = now.Add(time.Duration(c.cfg.RecoveryInitialDelaySeconds) * time.Second)
}

// Called only while holding c.mu (usually through update).
func (c *Controller) resetRecovery(message string) {
	if c.state.Recovery.Candidate != "" {
		addEvent(&c.state, "info", "恢复候选已重置："+message)
	}
	c.state.Recovery.Candidate = ""
	c.state.Recovery.Successes = 0
	c.state.Recovery.Message = message
}

func (c *Controller) observeRecoveryGroup(rev uint64, group ProxyGroup) {
	c.update(rev, func(s *Status) {
		if !c.cfg.RecoveryEnabled {
			return
		}
		if c.recovery.current != "" && (c.recovery.current != group.Now || !slices.Equal(c.recovery.order, group.All)) {
			c.resetRecovery("当前选择或分组顺序变化")
			if c.recovery.current != group.Now {
				c.restartRecoveryWait(time.Now())
			}
		}
		c.recovery.current = group.Now
		c.recovery.order = slices.Clone(group.All)
	})
}

func (c *Controller) recoveryError(rev uint64, err error) {
	c.update(rev, func(s *Status) {
		c.resetRecovery("恢复检查失败，重新累计")
		addEvent(s, "warning", "恢复检查失败："+err.Error())
	})
}

// Runs under the same cycle lock as the normal probe, after it has succeeded.
func (c *Controller) checkRecovery(ctx context.Context, cfg Config, rev uint64, client *Mihomo, group ProxyGroup) {
	if !cfg.RecoveryEnabled || ctx.Err() != nil {
		return
	}
	due := false
	var startedAt time.Time
	c.update(rev, func(s *Status) {
		if time.Now().Before(c.recovery.startedAt.Add(time.Duration(cfg.RecoveryInitialDelaySeconds) * time.Second)) {
			return
		}
		if time.Now().Before(c.recovery.nextCheck) {
			return
		}
		due = true
		c.recovery.checked = true
		startedAt = c.recovery.startedAt
		now := time.Now()
		s.Recovery.LastCheck = &now
	})
	if !due {
		return
	}
	// Even slow checks must wait a full interval before starting another round.
	defer c.update(rev, func(s *Status) {
		if c.recovery.startedAt.Equal(startedAt) {
			c.recovery.nextCheck = time.Now().Add(time.Duration(cfg.RecoveryIntervalSeconds) * time.Second)
		}
	})
	index := slices.Index(group.All, group.Now)
	if index <= 0 {
		c.update(rev, func(s *Status) {
			message := "当前已是最高优先级分组"
			if index < 0 {
				message = "当前选择不在原始顺序中，跳过恢复检查"
			}
			c.resetRecovery(message)
		})
		return
	}
	site := c.sites[rand.IntN(len(c.sites))]
	candidate := ""
	for _, name := range group.All[:index] {
		delay, err := client.ProbeGroup(ctx, name, site)
		c.recordGroupProbe(rev, group.All, name, delay, err)
		if err == nil {
			candidate = name
			break
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.Node {
			c.recoveryError(rev, err)
			return
		}
	}
	if candidate == "" {
		c.update(rev, func(s *Status) { c.resetRecovery("高优先级分组尚未恢复") })
		return
	}
	ready := false
	if !c.update(rev, func(s *Status) {
		if s.Recovery.Candidate != candidate {
			c.resetRecovery("首个可用候选发生变化")
			s.Recovery.Candidate = candidate
			addEvent(s, "info", "发现恢复候选："+candidate)
		}
		s.Recovery.Successes = min(s.Recovery.Successes+1, cfg.RecoverySuccessThreshold)
		s.Recovery.Message = fmt.Sprintf("%s 连续恢复成功 %d/%d 次", candidate, s.Recovery.Successes, cfg.RecoverySuccessThreshold)
		ready = s.Recovery.Successes >= cfg.RecoverySuccessThreshold
	}) || !ready || ctx.Err() != nil {
		return
	}
	// Probe first, then read the selection/order as late as possible before PUT.
	delay, err := client.ProbeGroup(ctx, candidate, site)
	c.recordGroupProbe(rev, group.All, candidate, delay, err)
	if err != nil {
		c.recoveryError(rev, err)
		return
	}
	latest, err := client.Group(ctx)
	if err != nil {
		c.recoveryError(rev, err)
		return
	}
	if latest.Now != group.Now || !slices.Equal(latest.All, group.All) {
		c.observeRecoveryGroup(rev, latest)
		c.update(rev, func(s *Status) { s.Current = latest.Now })
		return
	}
	if ctx.Err() != nil {
		return
	}
	if err = client.Select(ctx, candidate); err != nil {
		observed, observeErr := client.Group(ctx)
		if observeErr != nil {
			c.recoveryError(rev, observeErr)
			return
		}
		c.observeRecoveryGroup(rev, observed)
		c.update(rev, func(s *Status) { s.Current = observed.Now })
		if observed.Now != candidate {
			c.recoveryError(rev, err)
			return
		}
	}
	if c.update(rev, func(s *Status) {
		now := time.Now()
		c.restartRecoveryWait(now)
		c.recovery.current = candidate
		c.resetRecovery("回切成功，重新等待恢复检查")
		s.Current = candidate
		s.LastSwitch = &now
		s.Failures = 0
		addEvent(s, "success", "已自动恢复到高优先级分组 "+candidate)
	}) {
		c.notify(ctx, cfg, rev, notificationRecovery, group.Now, candidate)
	}
}
