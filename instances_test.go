package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testManager(t *testing.T, dir string) *InstanceManager {
	t.Helper()
	m, err := NewInstanceManager(dir, []string{"https://test.example/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestInstancesStartFreshAndIsolateConfigHistory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.json", "history.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old data, must not be read"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := testManager(t, dir)
	firstID := m.List()[0].ID
	first, _ := m.Get(firstID)
	if !validInstanceID(firstID) || firstID == "default" || first.controller.Config() != DefaultConfig() || first.controller.Status().EventCount != 0 {
		t.Fatal("first instance did not start with fresh settings")
	}
	if _, err := os.Stat(filepath.Join(dir, "instances", firstID, "config.json")); err != nil {
		t.Fatal("first instance did not use uniform storage")
	}
	item, err := m.Add("办公室")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := m.Get(item.ID)
	if second.controller.Config() != DefaultConfig() {
		t.Fatal("new instance inherited settings")
	}
	addEvent(&first.controller.state, "info", "first event")
	addEvent(&second.controller.state, "info", "second event")
	if first.controller.Status().Events[0].Message != "first event" {
		t.Fatal("histories mixed")
	}
	if err = m.Rename(item.ID, "家里"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	reloaded := testManager(t, dir)
	if len(reloaded.List()) != 2 || reloaded.List()[0].ID != firstID || reloaded.List()[1].Name != "家里" {
		t.Fatal("new registry not restored")
	}
	for _, name := range []string{"config.json", "history.json"} {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		if string(data) != "old data, must not be read" {
			t.Fatal("old data was modified")
		}
	}
}

func TestInstanceScopedAPIConfigSecretsAndPagination(t *testing.T) {
	m := testManager(t, t.TempDir())
	firstID := m.List()[0].ID
	item, err := m.Add("第二台")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewInstancesServer(m)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	w := request("PUT", "/api/instances/"+item.ID+"/config", `{"secret":"second-private","interval_seconds":9,"persist_history":true}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), "second-private") {
		t.Fatalf("scoped save failed/leaked: %d %s", w.Code, w.Body.String())
	}
	a, _ := m.Get(firstID)
	b, _ := m.Get(item.ID)
	if a.controller.Config().Secret != "" || b.controller.Config().Secret != "second-private" {
		t.Fatal("scoped secret changed wrong instance")
	}
	for i := 0; i < 120; i++ {
		addEvent(&b.controller.state, "info", "B only")
	}
	var page EventPage
	w = request("GET", "/api/instances/"+item.ID+"/events?limit=100", "")
	json.Unmarshal(w.Body.Bytes(), &page)
	if w.Code != 200 || len(page.Events) != 100 || !page.HasMore {
		t.Fatal("scoped pagination failed")
	}
	w = request("GET", "/api/instances/"+firstID+"/events", "")
	json.Unmarshal(w.Body.Bytes(), &page)
	if len(page.Events) != 0 {
		t.Fatal("events leaked into default")
	}
	w = request("GET", "/api/config", "")
	if w.Code != 404 {
		t.Fatal("unscoped legacy API is still available")
	}
	w = request("GET", "/api/instances", "")
	if strings.Contains(w.Body.String(), "second-private") {
		t.Fatal("registry response leaked secret")
	}
	w = request("PUT", "/api/instances/missing/config", `{"enabled":true}`)
	if w.Code != http.StatusNotFound {
		t.Fatal("unknown instance modified default")
	}
}

func TestInstanceRegistryValidationAndFailedWrite(t *testing.T) {
	dir := t.TempDir()
	m := testManager(t, dir)
	firstID := m.List()[0].ID
	for _, name := range []string{"", "默认实例", "bad\nname", strings.Repeat("字", 81)} {
		if _, err := m.Add(name); err == nil {
			t.Fatalf("accepted name %q", name)
		}
	}
	if err := m.Delete(firstID); err == nil {
		t.Fatal("deleted last instance")
	}
	if err := os.Remove(filepath.Join(dir, "instances.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "instances.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add("写入失败"); err == nil || len(m.List()) != 1 {
		t.Fatal("failed write changed registry")
	}
	m.Close()
	for _, content := range []string{`{"version":1,"instances":[{"id":"../escape","name":"bad"}]}`, `{"version":1,"instances":[]}`, `{bad`, `{}`, `null`} {
		broken := t.TempDir()
		os.WriteFile(filepath.Join(broken, "instances.json"), []byte(content), 0600)
		if mgr, err := NewInstanceManager(broken, nil); err == nil {
			mgr.Close()
			t.Fatal("accepted invalid registry")
		}
	}
}

func TestInstancesRunIndependentlyAndDeleteCancelsBlockedProbe(t *testing.T) {
	m := testManager(t, t.TempDir())
	firstID := m.List()[0].ID
	item, err := m.Add("正常实例")
	if err != nil {
		t.Fatal(err)
	}
	slow, slowAPI := newFake(t)
	slow.block = make(chan struct{})
	fast, fastAPI := newFake(t)
	fast.probeStatus = 200
	a, _ := m.Get(firstID)
	b, _ := m.Get(item.ID)
	cfg := testConfig(slowAPI.URL)
	cfg.IntervalSeconds = 1
	a.controller.Configure(cfg)
	cfg.APIURL = fastAPI.URL
	b.controller.Configure(cfg)
	m.Start()
	deadline := time.Now().Add(3 * time.Second)
	for b.controller.Status().Phase != "healthy" {
		if time.Now().After(deadline) {
			t.Fatal("slow instance blocked healthy instance")
		}
		time.Sleep(10 * time.Millisecond)
	}
	done := make(chan error, 1)
	go func() { done <- m.Delete(firstID) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("delete did not cancel blocked probe")
	}
	if _, ok := m.Get(firstID); ok {
		t.Fatal("deleted instance still registered")
	}
	w := httptest.NewRecorder()
	a.serve(w, httptest.NewRequest("GET", "http://localhost/api/config", nil))
	if w.Code != 410 {
		t.Fatal("deleted instance still accepts configuration")
	}
	if !b.controller.Config().Enabled {
		t.Fatal("deleting one instance stopped another")
	}
	m.Close()
}

func TestInstancesPersistIndependentHistoriesAndConfigAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	m := testManager(t, dir)
	firstID := m.List()[0].ID
	item, err := m.Add("第二台")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewInstancesServer(m)
	for _, id := range []string{firstID, item.ID} {
		req := httptest.NewRequest("PUT", "http://localhost/api/instances/"+id+"/config", strings.NewReader(`{"persist_history":true,"secret":"private-`+id+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		in, _ := m.Get(id)
		addEvent(&in.controller.state, "info", id)
	}
	m.Close()
	reloaded := testManager(t, dir)
	for _, id := range []string{firstID, item.ID} {
		in, ok := reloaded.Get(id)
		if !ok || in.controller.Config().Secret != "private-"+id || in.controller.Status().EventCount != 1 || in.controller.Status().Events[0].Message != id {
			t.Fatal("instance config/history did not restore independently")
		}
	}
}

func TestInstanceRegistryFailedRenameAndDeleteLeaveStateUnchanged(t *testing.T) {
	dir := t.TempDir()
	m := testManager(t, dir)
	item, err := m.Add("第二台")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "instances.json")
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	if m.Rename(item.ID, "新名称") == nil || m.List()[1].Name != "第二台" {
		t.Fatal("failed rename changed memory")
	}
	if m.Delete(item.ID) == nil || len(m.List()) != 2 {
		t.Fatal("failed deletion removed instance")
	}
}

func TestInstanceManagementRejectsCrossOriginAndInvalidJSON(t *testing.T) {
	m := testManager(t, t.TempDir())
	handler := NewInstancesServer(m)
	for _, body := range []string{`null`, `{"name":"x"} trailing`, `{"name":"x","secret":"bad"}`} {
		r := httptest.NewRequest("POST", "http://localhost/api/instances", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("invalid input accepted: %d", w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/api/instances", strings.NewReader(`{"name":"x"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 || len(m.List()) != 1 {
		t.Fatal("management API allowed cross-origin write")
	}
}

func TestInstanceDeleteCancelsInFlightConnectionTest(t *testing.T) {
	m := testManager(t, t.TempDir())
	item, err := m.Add("慢连接")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer api.Close()
	handler := NewInstancesServer(m)
	req := httptest.NewRequest("POST", "http://localhost/api/instances/"+item.ID+"/groups", strings.NewReader(`{"api_url":"`+api.URL+`","timeout_seconds":32}`))
	req.Header.Set("Content-Type", "application/json")
	draftDone := make(chan struct{})
	go func() { defer close(draftDone); handler.ServeHTTP(httptest.NewRecorder(), req) }()
	<-entered
	deleted := make(chan error, 1)
	go func() { deleted <- m.Delete(item.ID) }()
	select {
	case err := <-deleted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		// Cancel the test request before failing, so cleanup cannot wait 34 seconds.
		api.CloseClientConnections()
		<-draftDone
		t.Fatal("delete waited for slow connection test instead of cancelling it")
	}
	select {
	case <-draftDone:
	case <-time.After(time.Second):
		t.Fatal("connection test was not cancelled")
	}
}

func TestControllerConcurrentStopDoesNotRevertSavedSettings(t *testing.T) {
	c := NewController(nil)
	defer c.Stop()
	old := DefaultConfig()
	updated := old
	updated.Secret = "updated-private"
	updated.PersistHistory = true
	for i := 0; i < 5000; i++ {
		c.Configure(old)
		ready := make(chan struct{})
		var done sync.WaitGroup
		done.Add(2)
		go func() { defer done.Done(); <-ready; c.Stop() }()
		go func() { defer done.Done(); <-ready; c.Configure(updated) }()
		close(ready)
		done.Wait()
		got := c.Config()
		if got.Secret != updated.Secret || !got.PersistHistory {
			t.Fatal("concurrent stop reverted updated settings")
		}
	}
}
