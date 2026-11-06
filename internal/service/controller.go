package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/gateway"
)

type Controller struct {
	mu             sync.Mutex
	cycle          sync.Mutex
	cfg            config.Config
	rev            uint64
	ctx            context.Context
	cancel         context.CancelFunc
	wake           chan struct{}
	pool           *SitePool
	state          domain.Status
	recovery       recoveryState
	allUnavailable bool
	instanceName   string
	stateVersion   uint64
	currentVersion uint64
	stateEpoch     string
}

func NewController(sites []string) *Controller {
	pool, _ := OpenSitePool(sites, "")
	return NewControllerWithPool(pool)
}
func NewControllerWithPool(pool *SitePool) *Controller {
	c := &Controller{pool: pool, wake: make(chan struct{}, 1), instanceName: "默认实例", stateEpoch: time.Now().UTC().Format(time.RFC3339Nano)}
	c.Configure(config.DefaultConfig())
	return c
}
func (c *Controller) SetInstanceName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instanceName = name
}
func (c *Controller) Configure(cfg config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configureLocked(cfg)
}

func (c *Controller) configureLocked(cfg config.Config) {
	if c.cancel != nil {
		c.cancel()
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	changedTarget := c.cfg.APIURL != cfg.APIURL || c.cfg.Secret != cfg.Secret || c.cfg.Group != cfg.Group
	startRecovery := changedTarget || (!c.cfg.Enabled && cfg.Enabled) || (!c.cfg.RecoveryEnabled && cfg.RecoveryEnabled)
	c.cfg = cfg
	c.rev++
	c.stateVersion++
	c.currentVersion++
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
	active, total := c.pool.Counts()
	c.state = domain.Status{Running: cfg.Enabled, Phase: phase, Message: msg, PoolSize: active, PoolTotal: total, PoolQuarantined: total - active, Results: []domain.GroupResult{}, Events: c.state.Events, EventCount: c.state.EventCount, HistoryID: historyID, EventSequence: c.state.EventSequence}
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
func (c *Controller) Status() domain.Status {
	return c.FilteredStatus(false)
}
func (c *Controller) FilteredStatus(failuresOnly bool) domain.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	pruneEvents(&c.state, time.Now())
	s := c.state
	s.StateVersion, s.StateEpoch = c.stateVersion, c.stateEpoch
	s.PoolSize, s.PoolTotal = c.pool.Counts()
	s.PoolQuarantined = s.PoolTotal - s.PoolSize
	s.Recovery.Enabled = c.cfg.Enabled && c.cfg.RecoveryEnabled
	s.Recovery.Threshold = c.cfg.RecoverySuccessThreshold
	if s.Recovery.Enabled {
		initialRemaining := time.Until(c.recovery.startedAt.Add(time.Duration(c.cfg.RecoveryInitialDelaySeconds) * time.Second))
		if initialRemaining > 0 {
			s.Recovery.InitialWaitSeconds = int((initialRemaining + time.Second - 1) / time.Second)
		}
	}
	s.Results = append([]domain.GroupResult{}, s.Results...)
	page := historyPage(&c.state, 0, eventPageSize, failuresOnly)
	s.Events, s.EventCount = page.Events, page.Total
	return s
}
func (c *Controller) Config() config.Config { c.mu.Lock(); defer c.mu.Unlock(); return c.cfg }
func (c *Controller) update(rev uint64, fn func(*domain.Status)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rev != rev || c.ctx.Err() != nil {
		return false
	}
	previousCurrent := c.state.Current
	fn(&c.state)
	c.stateVersion++
	if c.state.Current != previousCurrent {
		c.currentVersion++
	}
	return true
}
func addEvent(s *domain.Status, level, message string) {
	now := time.Now()
	pruneEvents(s, now)
	s.EventSequence++
	s.Message = message
	s.Events = append([]domain.Event{{ID: s.EventSequence, Time: now, Level: level, Message: message}}, s.Events...)
	s.EventCount = len(s.Events)
}
func (c *Controller) fail(rev uint64, err error) {
	c.update(rev, func(s *domain.Status) {
		c.resetRecovery("API 错误中断连续恢复验证")
		s.Phase = "error"
		addEvent(s, "error", err.Error())
	})
}

// Merge a recovery probe without changing untested results or their timestamps.
// Control API failures do not establish whether the proxy itself is available.
func (c *Controller) recordGroupProbe(rev uint64, order []string, name string, delay int, err error) {
	if err != nil {
		var apiErr *gateway.APIError
		if !errors.As(err, &apiErr) || !apiErr.Node {
			return
		}
		delay = 0
	}
	c.update(rev, func(s *domain.Status) {
		previous := make(map[string]domain.GroupResult, len(s.Results))
		for _, result := range s.Results {
			previous[result.Name] = result
		}
		results := make([]domain.GroupResult, 0, len(order))
		now := time.Now()
		for _, member := range order {
			result, ok := previous[member]
			if !ok {
				result = domain.GroupResult{Name: member}
			}
			if member == name {
				result = domain.GroupResult{Name: name, Delay: delay, Available: err == nil, CheckedAt: &now, Node: result.Node}
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
	if !cfg.Enabled {
		return
	}
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	if ctx.Err() != nil {
		return
	}
	client := gateway.NewMihomo(cfg)
	c.retryQuarantinedSite(ctx, cfg, rev, client)
	if ctx.Err() != nil {
		return
	}
	choices := c.pool.Pick("", 1)
	if len(choices) == 0 {
		c.update(rev, func(s *domain.Status) {
			s.Phase = "waiting"
			s.Message = "有效站点池为空，等待隔离站点低频复测恢复"
		})
		return
	}
	site := choices[0]
	if !c.update(rev, func(s *domain.Status) {
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
	c.update(rev, func(s *domain.Status) { s.Current = group.Now })
	c.observeRecoveryGroup(rev, group)
	delay, err := client.Probe(ctx, site)
	if err == nil || isNodeFailure(err) {
		// Commit the observation now; only network rechecks wait for failover.
		if token := c.recordSiteOutcome(ctx, rev, site, err == nil); token != 0 {
			defer c.reviewSite(ctx, cfg, rev, site, token)
		}
	}
	if err == nil {
		c.update(rev, func(s *domain.Status) {
			c.allUnavailable = false
			s.Failures = 0
			s.Delay = delay
			s.Phase = "healthy"
			addEvent(s, "success", "当前出口探查成功；站点："+site)
		})
		c.checkRecovery(ctx, cfg, rev, client, group)
		return
	}
	var apiErr *gateway.APIError
	if !errors.As(err, &apiErr) || !apiErr.Node {
		c.fail(rev, err)
		return
	}
	trigger := false
	if !c.update(rev, func(s *domain.Status) {
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
	if c.recheckBaselineExit(ctx, rev, client, group) {
		return
	}
	var innerGuard gateway.ProxyGroup
	if c.recoverCurrentGroup(ctx, rev, client, group, site, &innerGuard) {
		return
	}
	if !c.update(rev, func(s *domain.Status) {
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
	// Do not apply old probe results over a manual selection or reloaded order.
	latestGroup, err := client.Group(ctx)
	if err != nil {
		c.fail(rev, err)
		return
	}
	if c.failureSelectionChanged(rev, group, latestGroup) {
		return
	}
	if innerGuard.Type != "" {
		if _, ok := c.checkedInnerGroup(ctx, rev, client, group, innerGuard); !ok {
			return
		}
	}
	group = latestGroup
	results := make([]domain.GroupResult, 0, len(group.All))
	checkedAt := time.Now()
	for _, name := range group.All {
		d, ok := delays[name]
		results = append(results, domain.GroupResult{Name: name, Delay: d, Available: ok && d > 0 && d <= 65535, CheckedAt: &checkedAt})
	}
	if !c.update(rev, func(s *domain.Status) {
		nodes := map[string]string{}
		for _, r := range s.Results {
			nodes[r.Name] = r.Node
		}
		for i := range results {
			results[i].Node = nodes[results[i].Name]
		}
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
			c.update(rev, func(s *domain.Status) {
				s.Failures = 0
				s.Phase = "healthy"
				addEvent(s, "success", "原始顺序中首个可用国家分组已在使用，保持当前选择")
			})
			return
		}
		if !c.update(rev, func(s *domain.Status) { s.Phase = "switching"; s.Message = "正在按原始顺序选择国家分组" }) {
			return
		}
		if innerGuard.Type != "" {
			if _, ok := c.checkedInnerGroup(ctx, rev, client, group, innerGuard); !ok {
				return
			}
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
				c.update(rev, func(s *domain.Status) {
					s.Current = observed.Now
					addEvent(s, "warning", "国家分组切换未成功，继续检查下一项")
				})
				continue
			}
		}
		if c.update(rev, func(s *domain.Status) {
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
			c.notify(ctx, cfg, rev, notificationFailover, group.Now, result.Name)
		}
		return
	}
	notifyUnavailable := false
	c.update(rev, func(s *domain.Status) {
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
		c.notify(ctx, cfg, rev, notificationUnavailable, group.Now, group.Now)
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

func (c *Controller) RecordEvent(level, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	addEvent(&c.state, level, message)
}
