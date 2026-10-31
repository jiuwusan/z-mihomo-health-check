package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const historyFlushInterval = 5 * time.Minute

type historySnapshot struct {
	Version   int     `json:"version"`
	HistoryID string  `json:"history_id"`
	Sequence  uint64  `json:"sequence"`
	Events    []Event `json:"events"`
}

type HistoryStore struct {
	mu         sync.Mutex
	controller *Controller
	filename   string
	persisted  bool
	lastHash   [32]byte
}

// Open once at startup, before starting the monitoring and persistence loops.
func OpenHistoryStore(c *Controller, filename string) (*HistoryStore, error) {
	store := &HistoryStore{controller: c, filename: filename}
	if !c.Config().PersistHistory {
		return store, nil
	}
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, errors.New("无法读取历史记录文件")
	}
	var snapshot historySnapshot
	if json.Unmarshal(data, &snapshot) != nil || snapshot.Version != 1 || snapshot.HistoryID == "" {
		return nil, errors.New("历史记录文件无效，请检查或备份后移走文件再启动")
	}
	seen := map[uint64]bool{}
	for _, event := range snapshot.Events {
		if event.ID == 0 || seen[event.ID] || event.Time.IsZero() {
			return nil, errors.New("历史记录文件包含无效或重复记录")
		}
		seen[event.ID] = true
		if event.ID > snapshot.Sequence {
			snapshot.Sequence = event.ID
		}
	}
	if snapshot.Events == nil {
		snapshot.Events = []Event{}
	}
	sort.Slice(snapshot.Events, func(i, j int) bool { return snapshot.Events[i].ID > snapshot.Events[j].ID })
	canonical, _ := json.Marshal(snapshot)
	store.lastHash = sha256.Sum256(canonical)
	store.persisted = true
	c.mu.Lock()
	c.state.Events = snapshot.Events
	c.state.HistoryID = snapshot.HistoryID
	c.state.eventSequence = snapshot.Sequence
	pruneEvents(&c.state, time.Now())
	c.mu.Unlock()
	return store, nil
}

func (c *Controller) historySnapshot() historySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	pruneEvents(&c.state, time.Now())
	return historySnapshot{Version: 1, HistoryID: c.state.HistoryID, Sequence: c.state.eventSequence, Events: append([]Event{}, c.state.Events...)}
}

// Serialize flushes and copy under the controller lock; disk I/O never blocks probes.
func (store *HistoryStore) Flush() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.controller.Config().PersistHistory {
		return nil
	}
	snapshot := store.controller.historySnapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return errors.New("历史记录序列化失败")
	}
	hash := sha256.Sum256(data)
	if store.persisted && hash == store.lastHash {
		return nil
	}
	if err = writeHistoryFile(store.filename, data); err != nil {
		return err
	}
	store.lastHash = hash
	store.persisted = true
	return nil
}

func writeHistoryFile(filename string, data []byte) error {
	if os.MkdirAll(filepath.Dir(filename), 0700) != nil {
		return errors.New("无法创建历史记录目录")
	}
	f, err := os.CreateTemp(filepath.Dir(filename), ".history-*")
	if err != nil {
		return errors.New("无法创建历史记录临时文件")
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("历史记录写入失败")
	}
	if os.Rename(temporary, filename) != nil {
		return errors.New("历史记录文件替换失败")
	}
	return nil
}

// The caller supplies a five-minute ticker; cancellation always flushes once more.
func (store *HistoryStore) Run(ctx context.Context, ticks <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return store.Flush()
		case _, ok := <-ticks:
			if !ok {
				return store.Flush()
			}
			if err := store.Flush(); err != nil {
				log.Printf("历史记录批量保存失败，将在下一周期重试：%v", err)
			}
		}
	}
}
