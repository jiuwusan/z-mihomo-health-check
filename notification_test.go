package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func webhookConfig(t *testing.T, cfg Config, hook string) Config {
	t.Helper()
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"dingtalk_webhook":`+quoteJSON(hook+"/ding")+`,"feishu_webhook":`+quoteJSON(hook+"/fei")+`}`))
	got, err := readInput(httptest.NewRecorder(), r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestWebhookConfigCredentials(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DingTalkWebhook = "https://oapi.dingtalk.com/robot/send?access_token=private-ding"
	cfg.FeishuWebhook = "https://open.feishu.cn/open-apis/bot/v2/hook/private-fei"
	filename := filepath.Join(t.TempDir(), "config.json")
	if err := SaveConfig(filename, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(filename)
	if err != nil || loaded != cfg {
		t.Fatalf("persistence failed: %v", err)
	}
	read := func(body string) Config {
		t.Helper()
		got, err := readInput(httptest.NewRecorder(), httptest.NewRequest("PUT", "/", strings.NewReader(body)), cfg)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, body := range []string{`{}`, `{"dingtalk_webhook":"","feishu_webhook":""}`} {
		got := read(body)
		if got != cfg {
			t.Fatal("omitted or empty webhook must retain credentials")
		}
	}
	got := read(`{"clear_dingtalk_webhook":true,"clear_feishu_webhook":true}`)
	if got.DingTalkWebhook != "" || got.FeishuWebhook != "" {
		t.Fatal("clear did not remove webhooks")
	}
	public, _ := json.Marshal(publicConfig(cfg))
	if strings.Contains(string(public), "private-") || publicConfig(cfg)["dingtalk_webhook_set"] != true || publicConfig(cfg)["feishu_webhook_set"] != true {
		t.Fatal("public config leaked credentials or omitted flags")
	}
	for _, hook := range []string{"not-url", "ftp://example.com", "https://user:password@example.com", "https://example.com/#secret"} {
		cfg.DingTalkWebhook = hook
		if cfg.Validate() == nil {
			t.Fatalf("accepted invalid webhook: %q", hook)
		}
	}
}

func TestWebhookFailureDoesNotUndoSwitchOrLeakCredentials(t *testing.T) {
	for _, response := range []struct {
		name, body string
		status     int
	}{
		{"http", "private-token", 500},
		{"ding-rejected", `{"errcode":310000,"errmsg":"private-token"}`, 200},
		{"fei-rejected", `{"code":19021,"msg":"private-token"}`, 200},
		{"malformed", `not-json`, 200},
		{"missing-code", `{}`, 200},
	} {
		t.Run(response.name, func(t *testing.T) {
			hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(response.status)
				w.Write([]byte(response.body))
			}))
			defer hook.Close()
			_, api := newFake(t)
			cfg := testConfig(api.URL)
			cfg.DingTalkWebhook = hook.URL + "/?token=private-token"
			c := testController(t, cfg)
			c.Check(context.Background())
			s := c.Status()
			if s.Current != "US" || s.Phase != "healthy" || !strings.Contains(s.Message, "已按原始顺序切换到") {
				t.Fatalf("notification altered switch: %+v", s)
			}
			found := false
			for _, event := range s.Events {
				found = found || strings.Contains(event.Message, "机器人通知发送失败")
				if strings.Contains(event.Message, "private-token") || strings.Contains(event.Message, hook.URL) {
					t.Fatal("notification error leaked credential")
				}
			}
			if !found {
				t.Fatal("missing failure event")
			}
		})
	}
}

func TestWebhookOneChannelFailureDoesNotSuppressOther(t *testing.T) {
	var mu sync.Mutex
	paths := map[string]int{}
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path]++
		mu.Unlock()
		if r.URL.Path == "/ding" {
			w.WriteHeader(503)
		} else {
			w.Write([]byte(`{"code":0}`))
		}
	}))
	defer hook.Close()
	_, api := newFake(t)
	c := testController(t, webhookConfig(t, testConfig(api.URL), hook.URL))
	c.Check(context.Background())
	mu.Lock()
	defer mu.Unlock()
	if paths["/ding"] != 1 || paths["/fei"] != 1 {
		t.Fatalf("channel suppressed: %v", paths)
	}
}

func TestWebhookCancellationAndTimeout(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer hook.Close()
	defer close(release)
	_, api := newFake(t)
	cfg := testConfig(api.URL)
	cfg.DingTalkWebhook = hook.URL
	c := testController(t, cfg)
	done := make(chan struct{})
	go func() { c.Check(context.Background()); close(done) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook request")
	}
	// Stop must remain responsive even when the remote robot hangs.
	c.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel webhook")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if sendWebhook(ctx, hook.URL, map[string]string{"msgtype": "text"}) == nil {
		t.Fatal("hanging request should time out")
	}
}
func quoteJSON(v string) string { b, _ := json.Marshal(v); return string(b) }
func TestWebhookNotifications(t *testing.T) {
	for _, scenario := range []string{"failover", "recovery", "unavailable", "selection_failed", "api_error", "unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			messages := map[string][]string{}
			hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("bad request")
				}
				var message string
				if r.URL.Path == "/ding" {
					if body["msgtype"] != "markdown" {
						t.Error("bad DingTalk format")
						return
					}
					message = body["markdown"].(map[string]any)["text"].(string)
					if body["markdown"].(map[string]any)["title"] != "Mihomo 节点状态变更" {
						t.Error("missing DingTalk title")
					}
				} else {
					if body["msg_type"] != "interactive" {
						t.Error("bad Feishu format")
						return
					}
					card := body["card"].(map[string]any)
					wantColor := "orange"
					if scenario == "recovery" {
						wantColor = "green"
					}
					if scenario == "unavailable" {
						wantColor = "red"
					}
					if card["header"].(map[string]any)["template"] != wantColor {
						t.Error("incorrect state color")
					}
					title := card["header"].(map[string]any)["title"].(map[string]any)["content"].(string)
					if title != "Mihomo 节点状态变更" {
						t.Error("missing card title")
					}
					message = title + "\n" + card["elements"].([]any)[0].(map[string]any)["text"].(map[string]any)["content"].(string)
				}
				mu.Lock()
				messages[r.URL.Path] = append(messages[r.URL.Path], message)
				mu.Unlock()
				w.Write([]byte(`{"errcode":0,"code":0}`))
			}))
			defer hook.Close()
			expected := ""
			if scenario == "recovery" {
				c, _ := recoveryFixture(t)
				c.Configure(webhookConfig(t, c.Config(), hook.URL))
				for i := 0; i < 3; i++ {
					recoveryCycle(c)
				}
				expected = "节点恢复"
			} else {
				f, api := newFake(t)
				c := testController(t, webhookConfig(t, testConfig(api.URL), hook.URL))
				switch scenario {
				case "failover":
					expected = "故障转移"
				case "unavailable":
					f.delays = map[string]int{}
					expected = "全节点故障"
				case "selection_failed":
					f.reject["US"] = true
					f.delays = map[string]int{"US": 90}
				case "api_error":
					f.groupStatus = 500
				case "unchanged":
					f.delays = map[string]int{"Japan": 100}
				}
				c.Check(context.Background())
				if scenario == "unavailable" {
					c.Check(context.Background())
					f.probeStatus = 200
					c.Check(context.Background())
					f.probeStatus = 504
					c.Check(context.Background())
				}
			}
			mu.Lock()
			defer mu.Unlock()
			for _, path := range []string{"/ding", "/fei"} {
				want := 0
				if expected != "" {
					want = 1
				}
				if scenario == "unavailable" {
					want = 2
				}
				if len(messages[path]) != want {
					t.Fatalf("%s: got %v want %d", path, messages[path], want)
				}
				for _, msg := range messages[path] {
					if !strings.Contains(msg, expected) || !strings.Contains(msg, "🚀 节点选择") || !strings.Contains(msg, "**名称：** 默认实例") || !strings.Contains(msg, "**新节点：**") {
						t.Fatalf("missing details: %s", msg)
					}
					parts := strings.Split(msg, "**时间：** ")
					if len(parts) != 2 {
						t.Fatal("missing notification time")
					}
					if _, err := time.Parse("2006-01-02 15:04:05", parts[1]); err != nil {
						t.Fatalf("invalid local timestamp: %v", err)
					}
				}
			}
		})
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
		if err := SaveConfig(filepath.Join(m.directory(info.ID), "config.json"), cfg); err != nil {
			t.Fatal(err)
		}
		f.current = "Japan"
		in.controller.Check(context.Background())
		if msg := <-messages; !strings.Contains(msg, "**名称：** "+info.Name) {
			t.Fatalf("wrong instance name: %s", msg)
		}
	}
	in, _ := m.Get(second.ID)
	before := in.controller.rev
	if err := m.Rename(second.ID, "家里"); err != nil {
		t.Fatal(err)
	}
	if in.controller.rev != before {
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

func TestNotificationEscapesMarkdownAndMentionTags(t *testing.T) {
	messages := make(chan struct{ path, text string }, 2)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Markdown struct {
				Text string `json:"text"`
			} `json:"markdown"`
			Card struct {
				Elements []struct {
					Text struct {
						Content string `json:"content"`
					} `json:"text"`
				} `json:"elements"`
			} `json:"card"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		text := body.Markdown.Text
		if r.URL.Path == "/fei" {
			text = body.Card.Elements[0].Text.Content
		}
		messages <- struct{ path, text string }{r.URL.Path, text}
		w.Write([]byte(`{"errcode":0,"code":0}`))
	}))
	defer hook.Close()
	cfg := webhookConfig(t, DefaultConfig(), hook.URL)
	cfg.Group = "组_[备用]"
	c := testController(t, cfg)
	c.SetInstanceName("家里 *主机* <at id=all> ~~备用~~")
	c.notify(context.Background(), cfg, c.rev, notificationFailover, "旧`节点", "新\n节点")
	for i := 0; i < 2; i++ {
		msg := <-messages
		wants := []string{"家里 \\*主机\\* &lt;at id=all&gt; \\~\\~备用\\~\\~", "组\\_\\[备用\\]", "旧\\`节点", "新 节点"}
		if msg.path == "/fei" {
			wants = []string{"家里 &#42;主机&#42; &#60;at id=all&#62; &#126;&#126;备用&#126;&#126;", "组&#95;&#91;备用&#93;", "旧&#96;节点", "新 节点"}
		}
		for _, want := range wants {
			if !strings.Contains(msg.text, want) {
				t.Fatalf("%s dynamic value was not escaped: %s", msg.path, msg.text)
			}
		}
	}
}
