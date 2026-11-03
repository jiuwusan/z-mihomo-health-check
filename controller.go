package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

type Event struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}
type GroupResult struct {
	Name      string     `json:"name"`
	Delay     int        `json:"delay_ms"`
	Available bool       `json:"available"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
}
type Status struct {
	Running       bool           `json:"running"`
	Phase         string         `json:"phase"`
	Message       string         `json:"message"`
	Current       string         `json:"current"`
	Failures      int            `json:"failures"`
	LastSite      string         `json:"last_site"`
	LastCheck     *time.Time     `json:"last_check"`
	LastSwitch    *time.Time     `json:"last_switch"`
	Delay         int            `json:"delay_ms"`
	PoolSize      int            `json:"pool_size"`
	Results       []GroupResult  `json:"results"`
	Events        []Event        `json:"events"`
	EventCount    int            `json:"event_count"`
	HistoryID     string         `json:"history_id"`
	Recovery      RecoveryStatus `json:"recovery"`
	eventSequence uint64
}
type Controller struct {
	mu             sync.Mutex
	cycle          sync.Mutex
	cfg            Config
	rev            uint64
	ctx            context.Context
	cancel         context.CancelFunc
	wake           chan struct{}
	sites          []string
	state          Status
	recovery       recoveryState
	allUnavailable bool
}

func NewController(sites []string) *Controller {
	c := &Controller{sites: append([]string(nil), sites...), wake: make(chan struct{}, 1)}
	c.Configure(DefaultConfig())
	return c
}
func (c *Controller) Configure(cfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configureLocked(cfg)
}

func (c *Controller) configureLocked(cfg Config) {
	if c.cancel != nil {
		c.cancel()
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	changedTarget := c.cfg.APIURL != cfg.APIURL || c.cfg.Secret != cfg.Secret || c.cfg.Group != cfg.Group
	startRecovery := changedTarget || (!c.cfg.Enabled && cfg.Enabled) || (!c.cfg.RecoveryEnabled && cfg.RecoveryEnabled)
	c.cfg = cfg
	c.rev++
	// Keep the observed baseline across saves so an unobserved manual switch
	// cannot bypass the initial wait. Progress is reset below with the public status.
	if changedTarget {
		c.recovery = recoveryState{}
		c.allUnavailable = false
	}
	c.recovery.nextCheck = time.Now().Add(time.Duration(cfg.RecoveryIntervalSeconds) * time.Second)
	if startRecovery || c.recovery.startedAt.IsZero() {
		c.restartRecoveryWait(time.Now())
	} else if !c.recovery.checked {
		c.recovery.nextCheck = c.recovery.startedAt.Add(time.Duration(cfg.RecoveryInitialDelaySeconds) * time.Second)
	}
	phase, msg := "stopped", "探查已停用"
	if cfg.Enabled {
		phase, msg = "waiting", "等待下一次探查"
	}
	pruneEvents(&c.state, time.Now())
	historyID := c.state.HistoryID
	if historyID == "" {
		historyID = time.Now().UTC().Format(time.RFC3339Nano)
	}
	c.state = Status{Running: cfg.Enabled, Phase: phase, Message: msg, PoolSize: len(c.sites), Results: []GroupResult{}, Events: c.state.Events, EventCount: c.state.EventCount, HistoryID: historyID, eventSequence: c.state.eventSequence}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg := c.cfg
	cfg.Enabled = false
	c.configureLocked(cfg)
}
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	pruneEvents(&c.state, time.Now())
	s := c.state
	s.Recovery.Enabled = c.cfg.Enabled && c.cfg.RecoveryEnabled
	s.Recovery.Threshold = c.cfg.RecoverySuccessThreshold
	if s.Recovery.Enabled {
		initialRemaining := time.Until(c.recovery.startedAt.Add(time.Duration(c.cfg.RecoveryInitialDelaySeconds) * time.Second))
		if initialRemaining > 0 {
			s.Recovery.InitialWaitSeconds = int((initialRemaining + time.Second - 1) / time.Second)
		}
	}
	s.Results = append([]GroupResult{}, s.Results...)
	s.Events = append([]Event{}, s.Events[:min(len(s.Events), eventPageSize)]...)
	return s
}
func (c *Controller) Config() Config { c.mu.Lock(); defer c.mu.Unlock(); return c.cfg }
func (c *Controller) update(rev uint64, fn func(*Status)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rev != rev || c.ctx.Err() != nil {
		return false
	}
	fn(&c.state)
	return true
}
func addEvent(s *Status, level, message string) {
	now := time.Now()
	pruneEvents(s, now)
	s.eventSequence++
	s.Message = message
	s.Events = append([]Event{{ID: s.eventSequence, Time: now, Level: level, Message: message}}, s.Events...)
	s.EventCount = len(s.Events)
}
func (c *Controller) fail(rev uint64, err error) {
	c.update(rev, func(s *Status) {
		c.resetRecovery("API 错误中断连续恢复验证")
		s.Phase = "error"
		addEvent(s, "error", err.Error())
	})
}

// Merge a recovery probe without changing untested results or their timestamps.
// Control API failures do not establish whether the proxy itself is available.
func (c *Controller) recordGroupProbe(rev uint64, order []string, name string, delay int, err error) {
	if err != nil {
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.Node {
			return
		}
		delay = 0
	}
	c.update(rev, func(s *Status) {
		previous := make(map[string]GroupResult, len(s.Results))
		for _, result := range s.Results {
			previous[result.Name] = result
		}
		results := make([]GroupResult, 0, len(order))
		now := time.Now()
		for _, member := range order {
			result, ok := previous[member]
			if !ok {
				result = GroupResult{Name: member}
			}
			if member == name {
				result = GroupResult{Name: name, Delay: delay, Available: err == nil, CheckedAt: &now}
			}
			results = append(results, result)
		}
		s.Results = results
	})
}

func (c *Controller) Check(parent context.Context) {
	if !c.cycle.TryLock() {
		return
	}
	defer c.cycle.Unlock()
	c.mu.Lock()
	cfg, rev, base := c.cfg, c.rev, c.ctx
	c.mu.Unlock()
	if !cfg.Enabled || len(c.sites) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	if ctx.Err() != nil {
		return
	}
	site := c.sites[rand.IntN(len(c.sites))]
	client := NewMihomo(cfg)
	if !c.update(rev, func(s *Status) {
		now := time.Now()
		s.LastCheck = &now
		s.LastSite = site
		s.Delay = 0
		s.Phase = "checking"
		s.Message = "正在测试当前出口"
	}) {
		return
	}
	group, err := client.Group(ctx)
	if err != nil {
		c.fail(rev, err)
		return
	}
	c.update(rev, func(s *Status) { s.Current = group.Now })
	c.observeRecoveryGroup(rev, group)
	delay, err := client.Probe(ctx, site)
	if err == nil {
		c.update(rev, func(s *Status) {
			c.allUnavailable = false
			s.Failures = 0
			s.Delay = delay
			s.Phase = "healthy"
			addEvent(s, "success", "当前出口探查成功")
		})
		c.checkRecovery(ctx, cfg, rev, client, group)
		return
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.Node {
		c.fail(rev, err)
		return
	}
	trigger := false
	if !c.update(rev, func(s *Status) {
		s.Failures++
		c.resetRecovery("当前出口失败，暂停恢复检查")
		s.Phase = "warning"
		addEvent(s, "warning", fmt.Sprintf("当前出口探查失败：%s；探查前选中分组：%s；站点：%s；超时：%ds", err, group.Now, site, cfg.TimeoutSeconds))
		trigger = s.Failures >= cfg.FailureThreshold
	}) {
		return
	}
	if !trigger {
		return
	}
	if !c.update(rev, func(s *Status) {
		s.Phase = "testing"
		addEvent(s, "info", "达到失败阈值，正在执行整组 delay 测试")
	}) {
		return
	}
	delays, err := client.Delays(ctx, site)
	if err != nil {
		c.fail(rev, err)
		return
	}
	// Re-read the active order after testing, in case the subscription was reloaded.
	group, err = client.Group(ctx)
	if err != nil {
		c.fail(rev, err)
		return
	}
	results := make([]GroupResult, 0, len(group.All))
	checkedAt := time.Now()
	for _, name := range group.All {
		d, ok := delays[name]
		results = append(results, GroupResult{Name: name, Delay: d, Available: ok && d > 0 && d <= 65535, CheckedAt: &checkedAt})
	}
	if !c.update(rev, func(s *Status) {
		s.Results = results
		s.Current = group.Now
		for _, result := range results {
			if result.Available {
				c.allUnavailable = false
				break
			}
		}
	}) {
		return
	}
	for _, result := range results {
		if !result.Available {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if group.Now == result.Name {
			c.update(rev, func(s *Status) {
				s.Failures = 0
				s.Phase = "healthy"
				addEvent(s, "success", "原始顺序中首个可用国家分组已在使用，保持当前选择")
			})
			return
		}
		if !c.update(rev, func(s *Status) { s.Phase = "switching"; s.Message = "正在按原始顺序选择国家分组" }) {
			return
		}
		err = client.Select(ctx, result.Name)
		// A lost HTTP response may still mean the switch was applied.
		if err != nil {
			observed, observeErr := client.Group(ctx)
			if observeErr != nil {
				c.fail(rev, observeErr)
				return
			}
			if observed.Now != result.Name {
				group = observed
				c.update(rev, func(s *Status) {
					s.Current = observed.Now
					addEvent(s, "warning", "国家分组切换未成功，继续检查下一项")
				})
				continue
			}
		}
		if c.update(rev, func(s *Status) {
			now := time.Now()
			c.restartRecoveryWait(now)
			c.resetRecovery("故障切换后重新等待恢复检查")
			c.recovery.current = result.Name
			s.LastSwitch = &now
			s.Current = result.Name
			s.Failures = 0
			s.Phase = "healthy"
			addEvent(s, "success", "已按原始顺序切换到 "+result.Name)
		}) {
			c.notify(ctx, cfg, rev, "节点故障，切换到可用节点", group.Now, result.Name)
		}
		return
	}
	notifyUnavailable := false
	c.update(rev, func(s *Status) {
		s.Phase = "error"
		addEvent(s, "error", "本轮没有可切换的国家分组，保留当前选择")
		available := false
		for _, result := range results {
			available = available || result.Available
		}
		if len(results) > 0 && !available && !c.allUnavailable {
			c.allUnavailable = true
			notifyUnavailable = true
		}
	})
	if notifyUnavailable {
		c.notify(ctx, cfg, rev, "节点故障，所有节点均不可用", group.Now, group.Now)
	}
}
func (c *Controller) Run(ctx context.Context) {
	for {
		cfg := c.Config()
		ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
		restart := false
		for !restart {
			select {
			case <-ctx.Done():
				ticker.Stop()
				c.Stop()
				return
			case <-c.wake:
				restart = true
			case <-ticker.C:
				c.Check(ctx)
				// Drop ticks that elapsed during a slow probe/failover; never queue a burst.
				draining := true
				for draining {
					select {
					case <-ticker.C:
					default:
						draining = false
					}
				}
			}
		}
		ticker.Stop()
	}
}
