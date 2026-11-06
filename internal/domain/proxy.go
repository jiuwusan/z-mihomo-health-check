package domain

type ProxyGroup struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Now            string   `json:"now"`
	All            []string `json:"all"`
	TestURL        string   `json:"testUrl,omitempty"`
	ExpectedStatus string   `json:"expectedStatus,omitempty"`
	Fixed          string   `json:"fixed,omitempty"`
}
