package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	c := testController(t, cfg)
	c.switchedAt = time.Now().Add(-time.Hour)
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

func TestRecoveryIntervalCooldownAndDisabled(t *testing.T) {
	c, f := recoveryFixture(t)
	c.Check(context.Background())
	if len(f.probes) != 1 {
		t.Fatal("recovery ran before interval")
	}
	c.switchedAt = time.Now()
	for i := 0; i < 3; i++ {
		recoveryCycle(c)
	}
	if len(f.selections) != 0 || c.Status().Recovery.CooldownSeconds < 295 {
		t.Fatal("cooldown bypassed")
	}
	c.switchedAt = time.Now().Add(-time.Hour)
	recoveryCycle(c)
	if f.current != "US" {
		t.Fatal("recovery did not resume after cooldown")
	}
	cfg := c.Config()
	cfg.RecoveryEnabled = false
	c.Configure(cfg)
	f.current = "Japan"
	f.probes = nil
	recoveryCycle(c)
	if len(f.probes) != 1 {
		t.Fatal("disabled recovery generated requests")
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
	if err != nil || cfg.RecoveryEnabled || cfg.RecoveryIntervalSeconds != 60 || cfg.RecoverySuccessThreshold != 3 || cfg.RecoveryCooldownSeconds != 300 {
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
	for _, bad := range []Config{func() Config { v := cfg; v.RecoveryIntervalSeconds = 0; return v }(), func() Config { v := cfg; v.RecoverySuccessThreshold = 0; return v }(), func() Config { v := cfg; v.RecoveryCooldownSeconds = -1; return v }()} {
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

func TestRecoveryCooldownDoesNotBlockFailover(t *testing.T) {
	f, s := newFake(t)
	cfg := testConfig(s.URL)
	cfg.RecoveryEnabled = true
	c := testController(t, cfg)
	c.Check(context.Background())
	if f.current != "US" || f.batches != 1 {
		t.Fatal("recovery cooldown blocked failover")
	}
	if c.Status().Recovery.CooldownSeconds < 295 {
		t.Fatal("failover did not start recovery cooldown")
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

func TestRecoveryManualSwitchRestartsCooldown(t *testing.T) {
	c, f := recoveryFixture(t)
	recoveryCycle(c)
	f.current = "Singapore"
	recoveryCycle(c)
	if len(f.selections) != 0 || c.Status().Recovery.Successes != 1 || c.Status().Recovery.CooldownSeconds < 295 {
		t.Fatal("manual selection did not restart validation and cooldown")
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
	if c.Status().Recovery.Successes != 1 || c.Status().Recovery.CooldownSeconds < 295 || len(f.selections) != 0 {
		t.Fatal("config save lost previous selection and bypassed manual-switch cooldown")
	}
}

func TestRecoveryChangedTargetRestartsCooldown(t *testing.T) {
	c, _ := recoveryFixture(t)
	cfg := c.Config()
	cfg.Group = "另一个分组"
	c.Configure(cfg)
	if c.Status().Recovery.CooldownSeconds < 295 {
		t.Fatal("changed target inherited expired cooldown")
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

func TestRecoveryLegacyConfigAndPartialAPIUpdate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(file, []byte(`{"api_url":"http://localhost:9090"}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(file)
	if err != nil || cfg.RecoveryEnabled || cfg.RecoveryIntervalSeconds != 60 || cfg.RecoverySuccessThreshold != 3 || cfg.RecoveryCooldownSeconds != 300 {
		t.Fatalf("legacy defaults lost: %+v %v", cfg, err)
	}
	c := testController(t, cfg)
	handler := NewServer(c, file)
	for _, body := range []string{`{"recovery_enabled":true,"recovery_interval_seconds":90,"recovery_success_threshold":4,"recovery_cooldown_seconds":600}`, `{"interval_seconds":7}`} {
		req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var got Config
		json.Unmarshal(w.Body.Bytes(), &got)
		saved, err := LoadConfig(file)
		if w.Code != 200 || err != nil || !got.RecoveryEnabled || got.RecoveryIntervalSeconds != 90 || got.RecoverySuccessThreshold != 4 || got.RecoveryCooldownSeconds != 600 || saved.RecoveryCooldownSeconds != 600 {
			t.Fatalf("recovery fields lost: %d %s %v", w.Code, w.Body.String(), err)
		}
	}
}
