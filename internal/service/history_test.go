package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/domain"
)

func record(c *Controller, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	addEvent(&c.state, "info", message)
}

func persistentController() *Controller {
	c := NewController(nil)
	cfg := c.Config()
	cfg.PersistHistory = true
	c.Configure(cfg)
	return c
}

func TestHistoryPersistenceDefaultsOffAndDoesNotReadOrWrite(t *testing.T) {
	if config.DefaultConfig().PersistHistory {
		t.Fatal("history persistence must default off")
	}
	file := filepath.Join(t.TempDir(), "history.json")
	original := []byte("existing unreadable snapshot must be ignored")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	c := NewController(nil)
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal("disabled history read old file:", err)
	}
	record(c, "memory only")
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	if string(data) != string(original) {
		t.Fatal("disabled persistence modified existing file")
	}
	if c.Events(0, 100).Total != 1 {
		t.Fatal("memory history unavailable while persistence disabled")
	}
	missing := filepath.Join(t.TempDir(), "history.json")
	store, err = OpenHistoryStore(c, missing)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("disabled persistence created history file")
	}
}

func TestHistoryPersistenceCanBeEnabledAndDisabledAtRuntime(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	c := NewController(nil)
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	record(c, "before enabling")
	cfg := c.Config()
	cfg.PersistHistory = true
	c.Configure(cfg)
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg.PersistHistory = false
	c.Configure(cfg)
	record(c, "memory after disable")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = store.Run(ctx, make(chan time.Time)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	if string(after) != string(saved) {
		t.Fatal("disabled shutdown wrote history")
	}
	if c.Events(0, 100).Total != 2 {
		t.Fatal("toggling persistence lost live memory history")
	}
}

func TestEnablingWithEmptyMemoryReplacesIgnoredSnapshot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(file, []byte("ignored while disabled"), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewController(nil)
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	cfg := c.Config()
	cfg.PersistHistory = true
	c.Configure(cfg)
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	restarted := persistentController()
	defer restarted.Stop()
	if _, err = OpenHistoryStore(restarted, file); err != nil {
		t.Fatal("enabling without events left invalid snapshot:", err)
	}
	if restarted.Events(0, 100).Total != 0 {
		t.Fatal("ignored old history unexpectedly restored")
	}
}

func TestConfigurationChangesAndStopPreserveHistory(t *testing.T) {
	c := NewController(nil)
	record(c, "before configuration change")
	before := c.Events(0, 100)
	cfg := c.Config()
	cfg.Group = "different group"
	cfg.IntervalSeconds = 7
	c.Configure(cfg)
	c.Stop()
	after := c.Events(0, 100)
	if len(after.Events) != 1 || after.Events[0].Message != "before configuration change" || after.HistoryID != before.HistoryID {
		t.Fatal("configuration change or stop discarded history")
	}
}

func TestHistoryRestoresAcrossRestartAndPreservesCursorIDs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	c := persistentController()
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	record(c, "first")
	record(c, "second")
	before := c.Events(0, 100)
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	restarted := persistentController()
	defer restarted.Stop()
	_, err = OpenHistoryStore(restarted, file)
	if err != nil {
		t.Fatal(err)
	}
	restored := restarted.Events(0, 100)
	if len(restored.Events) != 2 || restored.Events[0].Message != "second" || restored.HistoryID != before.HistoryID {
		t.Fatalf("restart did not restore history: %+v", restored)
	}
	record(restarted, "third")
	after := restarted.Events(0, 100)
	if after.Events[0].ID <= before.Events[0].ID {
		t.Fatal("event cursor IDs reused after restart")
	}
}

func TestHistoryBatchesWritesAndFlushesOnGracefulStop(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	c := persistentController()
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	done := make(chan error, 1)
	go func() { done <- store.Run(ctx, ticks) }()
	record(c, "buffered")
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("record was written before batch tick")
	}
	ticks <- time.Now()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err = os.Stat(file); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("batch tick did not persist history")
		}
		time.Sleep(time.Millisecond)
	}
	data, _ := os.ReadFile(file)
	var saved historySnapshot
	if json.Unmarshal(data, &saved) != nil || len(saved.Events) != 1 {
		t.Fatal("wrong saved snapshot")
	}
	record(c, "shutdown flush")
	cancel()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not flush")
	}
	restored := persistentController()
	defer restored.Stop()
	_, err = OpenHistoryStore(restored, file)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Events(0, 100).Total != 2 {
		t.Fatal("last buffered event lost on graceful stop")
	}
}

func TestHistoryLoadAndFlushDiscardExpiredRecords(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	now := time.Now()
	snapshot := historySnapshot{Version: 1, HistoryID: "saved-history", Sequence: 9, Events: []domain.Event{{ID: 9, Time: now.Add(-2 * time.Hour), Message: "expired"}}}
	data, _ := json.Marshal(snapshot)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	c := persistentController()
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Events(0, 100).Total != 0 {
		t.Fatal("expired records restored")
	}
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(file)
	var cleaned historySnapshot
	json.Unmarshal(data, &cleaned)
	if len(cleaned.Events) != 0 {
		t.Fatal("expired records remained on disk after flush")
	}
	record(c, "new")
	if c.Events(0, 100).Events[0].ID <= 9 {
		t.Fatal("expired snapshot sequence not preserved")
	}
}

func TestHistoryUnchangedSnapshotDoesNotRewriteFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	c := persistentController()
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	record(c, "one")
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err = os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(file)
	if info.ModTime().After(old.Add(time.Second)) {
		t.Fatal("unchanged history caused an unnecessary disk write")
	}
}

func TestCorruptHistoryIsNotSilentlyOverwritten(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.json")
	os.WriteFile(file, []byte(`{"broken":`), 0600)
	c := persistentController()
	defer c.Stop()
	if _, err := OpenHistoryStore(c, file); err == nil {
		t.Fatal("corrupt history accepted")
	}
	data, _ := os.ReadFile(file)
	if string(data) != `{"broken":` {
		t.Fatal("corrupt file overwritten")
	}
}

func TestHistoryRetriesAfterFailedDiskWrite(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "data")
	file := filepath.Join(folder, "history.json")
	c := persistentController()
	defer c.Stop()
	store, err := OpenHistoryStore(c, file)
	if err != nil {
		t.Fatal(err)
	}
	record(c, "not yet saved")
	if err = os.WriteFile(folder, []byte("blocks the data directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if store.Flush() == nil {
		t.Fatal("disk write failure was ignored")
	}
	if err = os.Remove(folder); err != nil {
		t.Fatal(err)
	}
	if err = store.Flush(); err != nil {
		t.Fatal(err)
	}
	restarted := persistentController()
	defer restarted.Stop()
	_, err = OpenHistoryStore(restarted, file)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Events(0, 100).Total != 1 {
		t.Fatal("failed write was incorrectly treated as persisted")
	}
}
