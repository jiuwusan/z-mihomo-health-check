package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recoveryAPI struct {
	current              string
	order                []string
	codes                map[string]int
	probes, selections   []string
	onProbe              func(string)
	selectCode           int
	applyOnFailure       bool
	groupCodeAfterSelect int
}

func recoveryFixture(t *testing.T) (*Controller, *recoveryAPI) {
	t.Helper()
	f := &recoveryAPI{current: "Japan", order: []string{"US", "Japan", "Singapore"}, codes: map[string]int{}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "PUT":
			var v struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&v)
			f.selections = append(f.selections, v.Name)
			if f.selectCode == 0 || f.applyOnFailure {
				f.current = v.Name
			}
			if f.selectCode != 0 {
				w.WriteHeader(f.selectCode)
				return
			}
			w.WriteHeader(204)
		case strings.HasSuffix(r.URL.Path, "/delay"):
			name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/proxies/"), "/delay")
			f.probes = append(f.probes, name)
			if f.onProbe != nil {
				f.onProbe(name)
			}
			if code := f.codes[name]; code != 0 {
				w.WriteHeader(code)
				return
			}
			json.NewEncoder(w).Encode(map[string]int{"delay": 123})
		default:
			if len(f.selections) > 0 && f.groupCodeAfterSelect != 0 {
				w.WriteHeader(f.groupCodeAfterSelect)
				return
			}
			json.NewEncoder(w).Encode(ProxyGroup{Type: "Selector", Now: f.current, All: f.order})
		}
	}))
	t.Cleanup(s.Close)
	cfg := DefaultConfig()
	cfg.APIURL = s.URL
	cfg.Secret = "test-secret"
	cfg.Enabled = true
	cfg.RecoveryEnabled = true
	cfg.RecoveryInitialDelaySeconds = 0
	c := testController(t, cfg)
	return c, f
}

func recoveryDue(c *Controller)   { c.recovery.nextCheck = time.Time{} }
func recoveryCycle(c *Controller) { recoveryDue(c); c.Check(context.Background()) }

func TestRecoveryRequiresConsecutiveSuccessAndRevalidates(t *testing.T) {
	c, f := recoveryFixture(t)
	for i := 1; i <= 3; i++ {
		recoveryCycle(c)
		if i < 3 && (len(f.selections) != 0 || c.Status().Recovery.Successes != i) {
			t.Fatalf("early switch / lost streak: %+v", c.Status().Recovery)
		}
	}
	if f.current != "US" || len(f.selections) != 1 || strings.Join(f.probes, ",") != "🚀 节点选择,US,🚀 节点选择,US,🚀 节点选择,US,US" {
		t.Fatalf("wrong recovery sequence: %+v", f)
	}
	if c.Status().Failures != 0 || c.Status().LastSwitch == nil {
		t.Fatal("recovery did not update switch status")
	}
}

func TestRecoveryIntervalAndDisabled(t *testing.T) {
	c, f := recoveryFixture(t)
	c.Check(context.Background())
	f.probes = nil
	c.Check(context.Background())
	if len(f.probes) != 1 || c.Status().Recovery.Successes != 1 {
		t.Fatal("recovery ignored interval")
	}
	cfg := c.Config()
	cfg.RecoveryEnabled = false
	c.Configure(cfg)
	f.probes = nil
	recoveryCycle(c)
	if len(f.probes) != 1 {
		t.Fatal("disabled recovery generated extra requests")
	}
}

func TestRecoveryResetsOnCandidateFailureAndAPIError(t *testing.T) {
	for _, code := range []int{503, 504, 401, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			c, f := recoveryFixture(t)
			recoveryCycle(c)
			f.codes["US"] = code
			recoveryCycle(c)
			if c.Status().Recovery.Successes != 0 || c.Status().Failures != 0 {
				t.Fatal("candidate error did not reset or affected current failure count")
			}
			delete(f.codes, "US")
			recoveryCycle(c)
			if c.Status().Recovery.Successes != 1 || len(f.selections) != 0 {
				t.Fatal("nonconsecutive success counted")
			}
		})
	}
}

func TestRecoveryOrderChangesAndManualSelectionCancel(t *testing.T) {
	for _, change := range []string{"order", "selection", "missing"} {
		t.Run(change, func(t *testing.T) {
			c, f := recoveryFixture(t)
			recoveryCycle(c)
			recoveryCycle(c)
			n := 0
			f.onProbe = func(name string) {
				if name != "US" {
					return
				}
				n++
				if n == 2 {
					switch change {
					case "order":
						f.order = []string{"Japan", "US", "Singapore"}
					case "selection":
						f.current = "Singapore"
					case "missing":
						f.order = []string{"US", "Singapore"}
					}
				}
			}
			recoveryCycle(c)
			if len(f.selections) != 0 || c.Status().Recovery.Successes != 0 {
				t.Fatal("selection/order change did not cancel recovery")
			}
		})
	}
}

func TestRecoveryFinalProbeFailureCancelsSwitch(t *testing.T) {
	c, f := recoveryFixture(t)
	recoveryCycle(c)
	recoveryCycle(c)
	n := 0
	f.onProbe = func(name string) {
		if name == "US" {
			n++
			if n == 2 {
				f.codes[name] = 504
			}
		}
	}
	recoveryCycle(c)
	if len(f.selections) != 0 || c.Status().Recovery.Successes != 0 {
		t.Fatal("failed final probe allowed switch")
	}
}

func TestRecoveryConfigurationDefaultsAndPersistence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	cfg, err := LoadConfig(file)
	if err != nil || cfg.RecoveryEnabled || cfg.RecoveryIntervalSeconds != 60 || cfg.RecoverySuccessThreshold != 3 {
		t.Fatalf("invalid defaults: %+v %v", cfg, err)
	}
	cfg.RecoveryEnabled = true
	if err = SaveConfig(file, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(file)
	if err != nil || got != cfg {
		t.Fatal("recovery configuration lost")
	}
	for _, bad := range []Config{func() Config { v := cfg; v.RecoveryIntervalSeconds = 0; return v }(), func() Config { v := cfg; v.RecoverySuccessThreshold = 0; return v }(), func() Config { v := cfg; v.RecoveryInitialDelaySeconds = -1; return v }()} {
		if bad.Validate() == nil {
			t.Fatal("invalid recovery setting accepted")
		}
	}
}

func TestRecoveryPrefersOriginalOrderAndResetsChangedCandidate(t *testing.T) {
	c, f := recoveryFixture(t)
	f.current = "Singapore"
	f.codes["US"] = 504
	recoveryCycle(c)
	if c.Status().Recovery.Candidate != "Japan" || c.Status().Recovery.Successes != 1 {
		t.Fatal("did not select first available original member")
	}
	delete(f.codes, "US")
	recoveryCycle(c)
	if c.Status().Recovery.Candidate != "US" || c.Status().Recovery.Successes != 1 {
		t.Fatal("candidate change retained previous streak")
	}
	if strings.Join(f.probes, ",") != "🚀 节点选择,US,Japan,🚀 节点选择,US" {
		t.Fatal("tested lower priority member or sorted candidates")
	}
}

func TestRecoveryInitialWaitDoesNotBlockFailover(t *testing.T) {
	f, s := newFake(t)
	cfg := testConfig(s.URL)
	cfg.RecoveryEnabled = true
	c := testController(t, cfg)
	c.Check(context.Background())
	if f.current != "US" || f.batches != 1 {
		t.Fatal("initial recovery wait blocked failover")
	}
	if c.Status().Recovery.InitialWaitSeconds < 295 {
		t.Fatal("failover did not restart initial recovery wait")
	}
}

func TestRecoveryCancelledByConfigurationChange(t *testing.T) {
	c, f := recoveryFixture(t)
	recoveryCycle(c)
	recoveryCycle(c)
	f.onProbe = func(name string) {
		if name == "US" {
			cfg := c.Config()
			cfg.Enabled = false
			c.Configure(cfg)
		}
	}
	recoveryCycle(c)
	if len(f.selections) != 0 || c.Status().Recovery.Successes != 0 {
		t.Fatal("cancelled recovery switched or retained progress")
	}
}

func TestRecoveryManualSwitchResetsProgress(t *testing.T) {
	c, f := recoveryFixture(t)
	recoveryCycle(c)
	f.current = "Singapore"
	recoveryCycle(c)
	if len(f.selections) != 0 || c.Status().Recovery.Successes != 1 {
		t.Fatal("manual selection did not restart validation")
	}
}

func TestRecoveryConfigSavePreservesManualSwitchDetection(t *testing.T) {
	c, f := recoveryFixture(t)
	recoveryCycle(c)
	f.current = "Singapore"
	cfg := c.Config()
	cfg.IntervalSeconds = 7
	c.Configure(cfg)
	recoveryCycle(c)
	if c.Status().Recovery.Successes != 1 || len(f.selections) != 0 {
		t.Fatal("config save lost previous selection and retained recovery progress")
	}
}

func TestRecoveryChangedTargetRestartsInitialWait(t *testing.T) {
	c, _ := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryInitialDelaySeconds = 300
	cfg.Group = "另一个分组"
	c.Configure(cfg)
	if c.Status().Recovery.InitialWaitSeconds < 295 {
		t.Fatal("changed target did not restart initial wait")
	}
}

func TestRecoverySwitchErrorConfirmsActualSelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		applied     bool
		groupCode   int
		wantCurrent string
		success     bool
	}{
		{"applied despite failed response", true, 0, "US", true},
		{"not applied", false, 0, "Japan", false},
		{"confirmation failed", false, 500, "Japan", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := recoveryFixture(t)
			f.selectCode = 503
			f.applyOnFailure = tc.applied
			f.groupCodeAfterSelect = tc.groupCode
			for i := 0; i < 3; i++ {
				recoveryCycle(c)
			}
			if len(f.selections) != 1 || c.Status().Current != tc.wantCurrent || c.Status().Recovery.Successes != 0 {
				t.Fatalf("wrong failed-switch handling: %+v", c.Status())
			}
			succeeded := false
			for _, event := range c.Status().Events {
				if strings.Contains(event.Message, "已自动恢复到高优先级分组") {
					succeeded = true
				}
			}
			if succeeded != tc.success {
				t.Fatal("incorrect switch result recorded")
			}
		})
	}
}

func TestRecoverySettingsAndPartialAPIUpdate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if err := SaveConfig(file, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(file)
	if err != nil || cfg.RecoveryEnabled || cfg.RecoveryIntervalSeconds != 60 || cfg.RecoverySuccessThreshold != 3 {
		t.Fatalf("default settings lost: %+v %v", cfg, err)
	}
	c := testController(t, cfg)
	handler := NewServer(c, file)
	for _, body := range []string{`{"recovery_enabled":true,"recovery_interval_seconds":90,"recovery_success_threshold":4}`, `{"interval_seconds":7}`} {
		req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var got Config
		json.Unmarshal(w.Body.Bytes(), &got)
		saved, err := LoadConfig(file)
		if w.Code != 200 || err != nil || !got.RecoveryEnabled || got.RecoveryIntervalSeconds != 90 || got.RecoverySuccessThreshold != 4 || saved.RecoveryIntervalSeconds != 90 {
			t.Fatalf("recovery fields lost: %d %s %v", w.Code, w.Body.String(), err)
		}
	}
}

func TestRecoveryInitialDelayBeforeFirstRoundOnly(t *testing.T) {
	c, f := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryInitialDelaySeconds = 300
	c.Configure(cfg)
	recoveryCycle(c)
	if len(f.probes) != 1 || c.Status().Recovery.InitialWaitSeconds < 295 {
		t.Fatal("first recovery ignored initial delay")
	}
	startedAt := c.recovery.startedAt
	cfg.IntervalSeconds = 7
	c.Configure(cfg)
	if !c.recovery.startedAt.Equal(startedAt) {
		t.Fatal("ordinary config save restarted initial wait")
	}
	c.recovery.startedAt = time.Now().Add(-301 * time.Second)
	recoveryCycle(c)
	if c.Status().Recovery.Successes != 1 {
		t.Fatal("first recovery did not run after initial delay")
	}
	c.Check(context.Background())
	if c.Status().Recovery.Successes != 1 {
		t.Fatal("subsequent round ignored recovery interval")
	}
	recoveryCycle(c)
	if c.Status().Recovery.Successes != 2 {
		t.Fatal("initial delay repeated between rounds")
	}
}

func TestRecoveryFirstRoundDoesNotWaitForSubsequentInterval(t *testing.T) {
	c, f := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryEnabled = false
	c.Configure(cfg)
	cfg.RecoveryEnabled = true
	cfg.RecoveryInitialDelaySeconds = 0
	cfg.RecoveryIntervalSeconds = 86400
	c.Configure(cfg)
	c.Check(context.Background())
	if len(f.probes) != 2 || c.Status().Recovery.Successes != 1 {
		t.Fatal("first round incorrectly waited for subsequent recovery interval")
	}
}

func TestRecoverySaveBeforeFirstRoundPreservesInitialSchedule(t *testing.T) {
	c, _ := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryInitialDelaySeconds = 300
	cfg.RecoveryIntervalSeconds = 86400
	c.Configure(cfg)
	c.recovery.startedAt = time.Now().Add(-60 * time.Second)
	cfg.PersistHistory = true
	c.Configure(cfg)
	if !c.recovery.nextCheck.Equal(c.recovery.startedAt.Add(300 * time.Second)) {
		t.Fatal("ordinary save replaced first-round deadline with subsequent interval")
	}
	c.recovery.startedAt = time.Now().Add(-301 * time.Second)
	c.Configure(cfg)
	c.Check(context.Background())
	if c.Status().Recovery.Successes != 1 {
		t.Fatal("first round still postponed after initial wait elapsed")
	}
}

func TestRecoverySuccessfulSwitchRestartsInitialDelay(t *testing.T) {
	c, f := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryInitialDelaySeconds = 300
	c.Configure(cfg)
	c.recovery.startedAt = time.Now().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		recoveryCycle(c)
	}
	if f.current != "US" || c.Status().Recovery.InitialWaitSeconds < 295 {
		t.Fatal("successful recovery did not restart initial wait")
	}
}

func TestRecoveryInitialDelayRestartsOnManualSwitchAndEnable(t *testing.T) {
	c, f := recoveryFixture(t)
	cfg := c.Config()
	cfg.RecoveryInitialDelaySeconds = 300
	c.Configure(cfg)
	c.recovery.startedAt = time.Now().Add(-time.Hour)
	recoveryCycle(c)
	f.current = "Singapore"
	f.probes = nil
	recoveryCycle(c)
	if len(f.probes) != 1 || c.Status().Recovery.Successes != 0 || c.Status().Recovery.InitialWaitSeconds < 295 {
		t.Fatal("manual switch did not restart first-round delay")
	}
	cfg.Enabled = false
	c.Configure(cfg)
	c.recovery.startedAt = time.Now().Add(-time.Hour)
	cfg.Enabled = true
	c.Configure(cfg)
	recoveryCycle(c)
	if c.Status().Recovery.InitialWaitSeconds < 295 || c.Status().Recovery.Successes != 0 {
		t.Fatal("enable did not restart initial delay")
	}
}

func TestRecoveryInitialDelayDefaultPersistenceAndValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if err := SaveConfig(file, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(file)
	if err != nil || cfg.RecoveryInitialDelaySeconds != 300 {
		t.Fatal("initial delay should default to 300")
	}
	c := testController(t, cfg)
	handler := NewServer(c, file)
	for _, body := range []string{`{"recovery_initial_delay_seconds":120}`, `{"interval_seconds":7}`} {
		req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var got Config
		json.Unmarshal(w.Body.Bytes(), &got)
		saved, err := LoadConfig(file)
		if w.Code != 200 || err != nil || got.RecoveryInitialDelaySeconds != 120 || saved.RecoveryInitialDelaySeconds != 120 {
			t.Fatal("initial delay lost on partial save")
		}
	}
	for _, n := range []int{-1, 86401} {
		cfg.RecoveryInitialDelaySeconds = n
		if cfg.Validate() == nil {
			t.Fatal("invalid initial delay accepted")
		}
	}
}

func TestRecoveryInitialWaitAndSuccessThresholdAllowSwitch(t *testing.T) {
	c, f := recoveryFixture(t)
	// A new switch must only be constrained by the configured initial delay,
	// which this fixture explicitly sets to zero.
	c.restartRecoveryWait(time.Now())
	for i := 0; i < 3; i++ {
		recoveryCycle(c)
	}
	if f.current != "US" || len(f.selections) != 1 {
		t.Fatal("validated recovery did not switch")
	}
}

func TestRemovedCooldownFieldRejected(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	cfg := DefaultConfig()
	c := testController(t, cfg)
	if err := SaveConfig(file, cfg); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(`{"recovery_cooldown_seconds":600,"interval_seconds":7}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	NewServer(c, file).ServeHTTP(w, req)
	if w.Code != 400 || c.Config() != cfg {
		t.Fatal("obsolete field still accepted")
	}
}
