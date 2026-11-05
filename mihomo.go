package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProxyGroup struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}
type APIError struct {
	Code    int
	Node    bool
	Message string
}

func (e *APIError) Error() string { return e.Message }

var apiTransport = func() *http.Transport { t := http.DefaultTransport.(*http.Transport).Clone(); t.Proxy = nil; return t }()

type Mihomo struct {
	cfg  Config
	http *http.Client
}

func NewMihomo(c Config) *Mihomo {
	return &Mihomo{c, &http.Client{Transport: apiTransport, Timeout: time.Duration(c.TimeoutSeconds+2) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (m *Mihomo) request(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	return m.requestLimited(ctx, method, path, q, body, out, 1024*1024)
}
func (m *Mihomo) requestLimited(ctx context.Context, method, path string, q url.Values, body any, out any, limit int64) error {
	endpoint := strings.TrimRight(m.cfg.APIURL, "/") + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return errors.New("API 请求地址无效")
	}
	if m.cfg.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+m.cfg.Secret)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &APIError{Message: "无法连接 Mihomo API，请检查地址、网络、超时或 TLS 证书"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("Mihomo API 返回 HTTP %d", resp.StatusCode)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			msg = "Mihomo API 认证失败，请检查 Secret"
		}
		return &APIError{Code: resp.StatusCode, Message: msg}
	}
	if out != nil {
		data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if err != nil || int64(len(data)) > limit || json.Unmarshal(data, out) != nil {
			return errors.New("Mihomo API 返回了无效或过大的 JSON")
		}
	}
	return nil
}
func (m *Mihomo) Group(ctx context.Context) (ProxyGroup, error) {
	var g ProxyGroup
	err := m.request(ctx, "GET", "/proxies/"+url.PathEscape(m.cfg.Group), nil, nil, &g)
	if err == nil && (g.Type != "Selector" || len(g.All) == 0) {
		err = errors.New("目标必须是包含成员的 select 分组")
	}
	g.Name = m.cfg.Group
	return g, err
}
func (m *Mihomo) Groups(ctx context.Context) ([]ProxyGroup, error) {
	var data struct {
		Proxies map[string]ProxyGroup `json:"proxies"`
	}
	err := m.request(ctx, "GET", "/proxies", nil, nil, &data)
	if err != nil {
		return nil, err
	}
	groups := []ProxyGroup{}
	for name, g := range data.Proxies {
		if g.Type == "Selector" && len(g.All) > 0 {
			g.Name = name
			groups = append(groups, g)
		}
	}
	return groups, nil
}
func (m *Mihomo) query(site string) url.Values {
	return url.Values{"url": {site}, "timeout": {fmt.Sprint(m.cfg.TimeoutSeconds * 1000)}, "expected": {"200-299"}}
}
func (m *Mihomo) Probe(ctx context.Context, site string) (int, error) {
	return m.ProbeGroup(ctx, m.cfg.Group, site)
}
func (m *Mihomo) ProbeGroup(ctx context.Context, name, site string) (int, error) {
	var data struct {
		Delay *int `json:"delay"`
	}
	err := m.request(ctx, "GET", "/proxies/"+url.PathEscape(name)+"/delay", m.query(site), nil, &data)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.Code == 503 || apiErr.Code == 504) {
			apiErr.Node = true
			apiErr.Message = fmt.Sprintf("当前出口测试失败（HTTP %d）", apiErr.Code)
		}
		return 0, err
	}
	if data.Delay == nil {
		return 0, errors.New("Mihomo API 响应缺少 delay 字段")
	}
	if *data.Delay <= 0 {
		return 0, &APIError{Node: true, Message: "当前出口测试失败（delay 为 0）"}
	}
	return *data.Delay, nil
}
func (m *Mihomo) Delays(ctx context.Context, site string) (map[string]int, error) {
	var data map[string]int
	err := m.request(ctx, "GET", "/group/"+url.PathEscape(m.cfg.Group)+"/delay", m.query(site), nil, &data)
	return data, err
}
func (m *Mihomo) Select(ctx context.Context, name string) error {
	return m.request(ctx, "PUT", "/proxies/"+url.PathEscape(m.cfg.Group), nil, map[string]string{"name": name}, nil)
}
