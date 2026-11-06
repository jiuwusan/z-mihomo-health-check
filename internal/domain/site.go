package domain

import (
	"time"
)

type SiteHealth struct {
	Failures      int       `json:"failures"`
	ReviewAfter   time.Time `json:"review_after,omitempty"`
	QuarantinedAt time.Time `json:"quarantined_at,omitempty"`
	RetryAt       time.Time `json:"retry_at,omitempty"`
}
type SitePoolFile struct {
	Version int                   `json:"version"`
	Sites   map[string]SiteHealth `json:"sites"`
}
