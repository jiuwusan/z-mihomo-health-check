package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"z-mihomo-health-check/internal/repository"
	"z-mihomo-health-check/internal/service"
)

func TestSiteChecksInstancesSharePoolAndRestartState(t *testing.T) {
	root := t.TempDir()
	m, err := NewInstanceManager(root, []string{"https://a.example/"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := m.Add("第二实例")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Get(info.ID)
	token := failSharedSite(t, m.pool, "https://a.example/", time.Now())
	if _, err := m.pool.FinishReview("https://a.example/", token, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if b.controller.Status().PoolSize != 0 {
		t.Fatal("other instance did not observe quarantine")
	}
	m.Close()
	reloaded, err := NewInstanceManager(root, []string{"https://a.example/"})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	in, _ := reloaded.Get(info.ID)
	if in.controller.Status().PoolSize != 0 {
		t.Fatal("restart reset quarantine")
	}
	if _, err := service.OpenSitePool([]string{"https://a.example/"}, filepath.Join(root, "site-pool.json")); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationInstanceNameOnCreateRenameAndReload(t *testing.T) {
	messages := make(chan string, 8)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Markdown struct {
				Text string `json:"text"`
			} `json:"markdown"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		messages <- body.Markdown.Text
		w.Write([]byte(`{"errcode":0}`))
	}))
	defer hook.Close()
	f, api := newFake(t)
	dir := t.TempDir()
	m := testManager(t, dir)
	second, err := m.Add("办公室")
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range m.List() {
		cfg := testConfig(api.URL)
		cfg.DingTalkWebhook = hook.URL
		in, _ := m.Get(info.ID)
		in.controller.Configure(cfg)
		if err := repository.SaveConfig(filepath.Join(m.directory(info.ID), "config.json"), cfg); err != nil {
			t.Fatal(err)
		}
		f.current = "Japan"
		in.controller.Check(context.Background())
		if msg := <-messages; !strings.Contains(msg, "**名称：** "+info.Name) {
			t.Fatalf("wrong instance name: %s", msg)
		}
	}
	in, _ := m.Get(second.ID)
	before := in.controller.Status().StateEpoch
	if err := m.Rename(second.ID, "家里"); err != nil {
		t.Fatal(err)
	}
	if in.controller.Status().StateEpoch != before {
		t.Fatal("rename reset running controller")
	}
	f.current = "Japan"
	in.controller.Check(context.Background())
	if msg := <-messages; !strings.Contains(msg, "**名称：** 家里") {
		t.Fatalf("rename did not update notification: %s", msg)
	}
	m.Close()
	reloaded := testManager(t, dir)
	in, _ = reloaded.Get(second.ID)
	f.current = "Japan"
	in.controller.Check(context.Background())
	if msg := <-messages; !strings.Contains(msg, "**名称：** 家里") {
		t.Fatalf("reload lost name: %s", msg)
	}
}

type fakeMihomo struct {
	mu              sync.Mutex
	probeStatus     int
	groupStatus     int
	delays          map[string]int
	current         string
	order           []string
	probes, batches int
	selections      []string
	reject          map[string]bool
	block           chan struct{}
}

func newFake(t *testing.T) (*fakeMihomo, *httptest.Server) {
	t.Helper()
	f := &fakeMihomo{probeStatus: 504, groupStatus: 200, current: "Japan", order: []string{"US", "Japan", "Singapore"}, delays: map[string]int{"US": 900, "Japan": 100}, reject: map[string]bool{}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/proxies":
			json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{"🚀 节点选择": map[string]any{"type": "Selector", "all": f.order, "now": f.current}}})
		case strings.HasPrefix(r.URL.Path, "/group/") && strings.HasSuffix(r.URL.Path, "/delay"):
			f.batches++
			if r.URL.Query().Get("expected") != "200-299" || r.URL.Query().Get("url") != "https://test.example/" || r.URL.Query().Get("timeout") != "4000" {
				t.Errorf("wrong query: %v", r.URL.Query())
			}
			w.WriteHeader(f.groupStatus)
			json.NewEncoder(w).Encode(f.delays)
		case r.Method == "PUT":
			var body struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.selections = append(f.selections, body.Name)
			if f.reject[body.Name] {
				w.WriteHeader(503)
				return
			}
			f.current = body.Name
			w.WriteHeader(204)
		case strings.HasPrefix(r.URL.Path, "/proxies/") && !strings.HasSuffix(r.URL.Path, "/delay") && r.URL.Path != "/proxies/🚀 节点选择":
			json.NewEncoder(w).Encode(map[string]any{"type": "Shadowsocks"})
		case strings.HasSuffix(r.URL.Path, "/delay"):
			f.probes++
			if f.block != nil {
				block := f.block
				f.mu.Unlock()
				select {
				case <-block:
				case <-r.Context().Done():
				}
				f.mu.Lock()
			}
			w.WriteHeader(f.probeStatus)
			if f.probeStatus == 200 {
				fmt.Fprint(w, `{"delay":123}`)
			}
		default:
			if r.URL.Path != "/proxies/🚀 节点选择" {
				t.Errorf("bad escaped group: %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]any{"type": "Selector", "all": f.order, "now": f.current})
		}
	}))
	t.Cleanup(s.Close)
	return f, s
}

func failSharedSite(t *testing.T, p *service.SitePool, url string, now time.Time) uint64 {
	t.Helper()
	var token uint64
	for i := 0; i < 20; i++ {
		var err error
		token, err = p.Record(url, false, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	return token
}
