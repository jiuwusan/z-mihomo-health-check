package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigPersistenceAndValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "data", "config.json")
	cfg := testConfig("http://localhost:9090")
	if err := SaveConfig(file, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(file)
	if err != nil || got != cfg {
		t.Fatalf("configuration not restored: %v %v", got, err)
	}
	cfg.IntervalSeconds = 0
	if SaveConfig(file, cfg) == nil {
		t.Fatal("invalid configuration persisted")
	}
	got, _ = LoadConfig(file)
	if got.IntervalSeconds != 5 {
		t.Fatal("invalid update destroyed previous config")
	}
	for _, apiURL := range []string{"file:///etc/passwd", "http://user:secret@localhost:9090", "http://localhost:9090/?token=secret", "not a URL"} {
		bad := got
		bad.APIURL = apiURL
		if bad.Validate() == nil {
			t.Errorf("accepted %q", apiURL)
		}
	}
}
func TestServerPreservesMasksAndExplicitlyClearsSecret(t *testing.T) {
	cfg := testConfig("http://localhost:9090")
	cfg.Enabled = false
	c := testController(t, cfg)
	file := filepath.Join(t.TempDir(), "config.json")
	s := httptest.NewServer(NewServer(c, file))
	defer s.Close()
	resp, err := http.Get(s.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if bytes.Contains(data, []byte("test-secret")) || !bytes.Contains(data, []byte(`"secret_set":true`)) {
		t.Fatalf("bad public config: %s", data)
	}
	put := func(body string) int {
		t.Helper()
		req, _ := http.NewRequest("PUT", s.URL+"/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if put(`{"interval_seconds":7,"secret":""}`) != 200 || c.Config().Secret != "test-secret" || c.Config().IntervalSeconds != 7 {
		t.Fatal("empty secret did not preserve saved credential")
	}
	restored, err := LoadConfig(file)
	if err != nil || restored.Secret != "test-secret" {
		t.Fatal("secret was not persisted")
	}
	if put(`{"clear_secret":true}`) != 200 || c.Config().Secret != "" {
		t.Fatal("explicit clear failed")
	}
	if put(`{"interval_seconds":0}`) != 400 || c.Config().IntervalSeconds != 7 {
		t.Fatal("bad request changed active config")
	}
	if put(`{"interval_seconds":8} trailing`) != 400 {
		t.Fatal("accepted trailing JSON")
	}
}
func TestDraftConnectionDoesNotSaveOrEnableMonitoring(t *testing.T) {
	_, api := newFake(t)
	c := NewController([]string{"https://test.example/"})
	defer c.Stop()
	file := filepath.Join(t.TempDir(), "config.json")
	s := httptest.NewServer(NewServer(c, file))
	defer s.Close()
	body, _ := json.Marshal(testConfig(api.URL))
	resp, err := http.Post(s.URL+"/api/groups", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.Contains(data, []byte("🚀 节点选择")) || bytes.Contains(data, []byte("test-secret")) {
		t.Fatalf("draft connection failed: %d %s", resp.StatusCode, data)
	}
	if c.Config().Enabled || c.Config().Secret != "" {
		t.Fatal("draft test modified config")
	}
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("draft test persisted config")
	}
}
func TestFailedSaveDoesNotApplyOrExposeCredentials(t *testing.T) {
	c := NewController([]string{"https://test.example/"})
	defer c.Stop()
	blocked := filepath.Join(t.TempDir(), "not-directory")
	os.WriteFile(blocked, []byte("x"), 0600)
	s := httptest.NewServer(NewServer(c, filepath.Join(blocked, "config.json")))
	defer s.Close()
	req, _ := http.NewRequest("PUT", s.URL+"/api/config", strings.NewReader(`{"secret":"private-value","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 500 || c.Config().Enabled || c.Config().Secret != "" || bytes.Contains(data, []byte("private-value")) {
		t.Fatal("failed save was applied or leaked secret")
	}
}
func TestServerRejectsCrossOriginWrites(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	s := httptest.NewServer(NewServer(c, filepath.Join(t.TempDir(), "config.json")))
	defer s.Close()
	req, _ := http.NewRequest("PUT", s.URL+"/api/config", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://untrusted.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin write accepted: %d", resp.StatusCode)
	}
}
func TestSchedulerRunsWithoutBrowserAndStopsAfterDisable(t *testing.T) {
	f, api := newFake(t)
	f.probeStatus = 200
	cfg := testConfig(api.URL)
	cfg.IntervalSeconds = 1
	c := testController(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock()
		n := f.probes
		f.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background monitor never ran")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cfg.Enabled = false
	c.Configure(cfg)
	time.Sleep(1100 * time.Millisecond)
	f.mu.Lock()
	n := f.probes
	f.mu.Unlock()
	if n != 1 {
		t.Fatalf("probe ran after disable: %d", n)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func TestTimeoutDoesNotExceedMihomoSigned16BitMilliseconds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TimeoutSeconds = 32
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.TimeoutSeconds = 33
	if cfg.Validate() == nil {
		t.Fatal("timeout over 32767 milliseconds was accepted")
	}
}

func TestServerRejectsMatchingButUntrustedHostAndOrigin(t *testing.T) {
	c := testController(t, testConfig("http://localhost:9090"))
	handler := NewServer(c, filepath.Join(t.TempDir(), "config.json"))
	req := httptest.NewRequest("POST", "http://evil.example:8080/api/groups", strings.NewReader(`{"api_url":"http://127.0.0.1:1"}`))
	req.Header.Set("Origin", "http://evil.example:8080")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("untrusted Host bypassed protection: %d", w.Code)
	}
}

func TestExplicitPanelHostnameAllowsReverseProxy(t *testing.T) {
	t.Setenv("PANEL_ALLOWED_HOSTS", "health.example.com")
	c := NewController(nil)
	defer c.Stop()
	handler := NewServer(c, filepath.Join(t.TempDir(), "config.json"))
	req := httptest.NewRequest("GET", "https://health.example.com/api/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("explicit reverse-proxy hostname rejected: %d", w.Code)
	}
}

func TestNullConfigurationRejected(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	handler := NewServer(c, filepath.Join(t.TempDir(), "config.json"))
	req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(`null`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("null configuration accepted: %d", w.Code)
	}
}

func TestHistoryPersistenceOptionRoundTripsThroughAPI(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	file := filepath.Join(t.TempDir(), "config.json")
	handler := NewServer(c, file)
	for _, body := range []string{`{"persist_history":true}`, `{"interval_seconds":7}`, `{"persist_history":false}`} {
		req := httptest.NewRequest("PUT", "http://localhost/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("option save failed: %d %s", w.Code, w.Body.String())
		}
		var public struct {
			PersistHistory bool `json:"persist_history"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &public); err != nil {
			t.Fatal(err)
		}
		expected := body != `{"persist_history":false}`
		saved, err := LoadConfig(file)
		if err != nil || saved.PersistHistory != expected || public.PersistHistory != expected {
			t.Fatal("option not retained across API/persistence")
		}
	}
}

func TestInitialPanelUsesNetworkServiceAndPreservesSavedAPIAddress(t *testing.T) {
	for _, tc := range []struct{ name, saved, want string }{
		{"fresh installation", "", "http://mihomo:9090"},
		{"saved loopback", "http://127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"saved custom service", "https://proxy.example.net/clash", "https://proxy.example.net/clash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.json")
			if tc.saved != "" {
				cfg := DefaultConfig()
				cfg.APIURL = tc.saved
				if err := SaveConfig(file, cfg); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := LoadConfig(file)
			if err != nil {
				t.Fatal(err)
			}
			c := NewController(nil)
			defer c.Stop()
			c.Configure(cfg)
			w := httptest.NewRecorder()
			NewServer(c, file).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/config", nil))
			var data struct {
				APIURL string `json:"api_url"`
			}
			if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || data.APIURL != tc.want {
				t.Fatalf("panel API address = %q, want %q", data.APIURL, tc.want)
			}
		})
	}
}
