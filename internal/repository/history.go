package repository

import (
	"encoding/json"
	"errors"
	"os"
	"sort"

	"z-mihomo-health-check/internal/domain"
)

type HistorySnapshot struct {
	Version   int            `json:"version"`
	HistoryID string         `json:"history_id"`
	Sequence  uint64         `json:"sequence"`
	Events    []domain.Event `json:"events"`
}

func LoadHistory(filename string) (*HistorySnapshot, error) {
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("无法读取历史记录文件")
	}
	var snapshot HistorySnapshot
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
		snapshot.Events = []domain.Event{}
	}
	sort.Slice(snapshot.Events, func(i, j int) bool { return snapshot.Events[i].ID > snapshot.Events[j].ID })
	return &snapshot, nil
}
