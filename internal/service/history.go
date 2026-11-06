package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/repository"
)

const HistoryFlushInterval = 5 * time.Minute

type historySnapshot = repository.HistorySnapshot

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
	saved, err := repository.LoadHistory(filename)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return store, nil
	}
	snapshot := *saved
	canonical, _ := json.Marshal(snapshot)
	store.lastHash = sha256.Sum256(canonical)
	store.persisted = true
	c.mu.Lock()
	c.state.Events = snapshot.Events
	c.state.HistoryID = snapshot.HistoryID
	c.state.EventSequence = snapshot.Sequence
	pruneEvents(&c.state, time.Now())
	c.mu.Unlock()
	return store, nil
}

func (c *Controller) historySnapshot() historySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	pruneEvents(&c.state, time.Now())
	return historySnapshot{Version: 1, HistoryID: c.state.HistoryID, Sequence: c.state.EventSequence, Events: append([]domain.Event{}, c.state.Events...)}
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
	if err = repository.WriteHistoryFile(store.filename, data); err != nil {
		return err
	}
	store.lastHash = hash
	store.persisted = true
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
