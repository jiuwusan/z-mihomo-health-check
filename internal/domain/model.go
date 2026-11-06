package domain

import (
	"time"
)

type Event struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}
type GroupResult struct {
	Name        string     `json:"name"`
	Delay       int        `json:"delay_ms"`
	Available   bool       `json:"available"`
	CheckedAt   *time.Time `json:"checked_at,omitempty"`
	Node        string     `json:"node,omitempty"`
	Source      string     `json:"source,omitempty"`
	StatusKnown bool       `json:"status_known,omitempty"`
}
type Status struct {
	StateVersion     uint64         `json:"state_version"`
	StateEpoch       string         `json:"state_epoch"`
	Running          bool           `json:"running"`
	Phase            string         `json:"phase"`
	Message          string         `json:"message"`
	Current          string         `json:"current"`
	Failures         int            `json:"failures"`
	LastSite         string         `json:"last_site"`
	LastCheck        *time.Time     `json:"last_check"`
	LastSwitch       *time.Time     `json:"last_switch"`
	Delay            int            `json:"delay_ms"`
	PoolSize         int            `json:"pool_size"`
	PoolTotal        int            `json:"pool_total"`
	PoolQuarantined  int            `json:"pool_quarantined"`
	Results          []GroupResult  `json:"results"`
	Events           []Event        `json:"events"`
	EventCount       int            `json:"event_count"`
	HistoryID        string         `json:"history_id"`
	Recovery         RecoveryStatus `json:"recovery"`
	NodeInfoSyncedAt *time.Time     `json:"node_info_synced_at,omitempty"`
	EventSequence    uint64         `json:"-"`
}
type RecoveryStatus struct {
	Enabled            bool       `json:"enabled"`
	Candidate          string     `json:"candidate"`
	Successes          int        `json:"successes"`
	Threshold          int        `json:"threshold"`
	InitialWaitSeconds int        `json:"initial_wait_seconds"`
	LastCheck          *time.Time `json:"last_check"`
	Message            string     `json:"message"`
}

type EventPage struct {
	Events           []Event `json:"events"`
	Total            int     `json:"total"`
	HasMore          bool    `json:"has_more"`
	NextBefore       uint64  `json:"next_before"`
	RetentionSeconds int     `json:"retention_seconds"`
	HistoryID        string  `json:"history_id"`
}

type InstanceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
