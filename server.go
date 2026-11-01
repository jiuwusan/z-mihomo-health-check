package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed web/*
var webFiles embed.FS

type server struct {
	controller *Controller
	filename   string
	save       sync.Mutex
}
type settingsInput struct {
	APIURL           string  `json:"api_url"`
	Secret           *string `json:"secret"`
	ClearSecret      bool    `json:"clear_secret"`
	Group            string  `json:"group"`
	IntervalSeconds  int     `json:"interval_seconds"`
	FailureThreshold int     `json:"failure_threshold"`
	TimeoutSeconds   int     `json:"timeout_seconds"`
	Enabled          bool    `json:"enabled"`
	PersistHistory   bool    `json:"persist_history"`
}

func inputFrom(c Config) settingsInput {
	return settingsInput{APIURL: c.APIURL, Group: c.Group, IntervalSeconds: c.IntervalSeconds, FailureThreshold: c.FailureThreshold, TimeoutSeconds: c.TimeoutSeconds, Enabled: c.Enabled, PersistHistory: c.PersistHistory}
}
func (in settingsInput) config(previous Config) Config {
	secret := previous.Secret
	if in.Secret != nil && *in.Secret != "" {
		secret = *in.Secret
	}
	if in.ClearSecret {
		secret = ""
	}
	return Config{APIURL: strings.TrimRight(strings.TrimSpace(in.APIURL), "/"), Secret: secret, Group: strings.TrimSpace(in.Group), IntervalSeconds: in.IntervalSeconds, FailureThreshold: in.FailureThreshold, TimeoutSeconds: in.TimeoutSeconds, Enabled: in.Enabled, PersistHistory: in.PersistHistory}
}
func publicConfig(c Config) map[string]any {
	return map[string]any{"api_url": c.APIURL, "secret_set": c.Secret != "", "group": c.Group, "interval_seconds": c.IntervalSeconds, "failure_threshold": c.FailureThreshold, "timeout_seconds": c.TimeoutSeconds, "enabled": c.Enabled, "persist_history": c.PersistHistory}
}
func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
func readInput(w http.ResponseWriter, r *http.Request, c Config) (Config, error) {
	initial := inputFrom(c)
	input := &initial
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return c, errors.New("配置内容不是有效的 JSON 对象，或存在未知字段")
	}
	if input == nil {
		return c, errors.New("配置必须是 JSON 对象，不能为 null")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return c, errors.New("请求只能包含一个 JSON 对象")
	}
	result := input.config(c)
	return result, result.Validate()
}
func NewServer(c *Controller, filename string) http.Handler {
	s := &server{controller: c, filename: filename}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, publicConfig(c.Config())) })
	mux.HandleFunc("PUT /api/config", s.configure)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, c.Status()) })
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("POST /api/groups", s.groups)
	assets, _ := fs.Sub(webFiles, "web")
	files := http.FileServer(http.FS(assets))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					writeError(w, 403, errors.New("拒绝跨来源配置请求"))
					return
				}
			}
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mediaType != "application/json" {
				writeError(w, 415, errors.New("请使用 application/json 请求"))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	limit := eventPageSize
	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > eventPageSize {
			writeError(w, 400, errors.New("每页记录数范围为 1–100"))
			return
		}
		limit = n
	}
	var before uint64
	if value := r.URL.Query().Get("before"); value != "" {
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			writeError(w, 400, errors.New("记录游标无效"))
			return
		}
		before = n
	}
	writeJSON(w, 200, s.controller.Events(before, limit))
}
func (s *server) configure(w http.ResponseWriter, r *http.Request) {
	s.save.Lock()
	defer s.save.Unlock()
	cfg, err := readInput(w, r, s.controller.Config())
	if err != nil {
		writeError(w, 400, err)
		return
	}
	if err = SaveConfig(s.filename, cfg); err != nil {
		writeError(w, 500, err)
		return
	}
	s.controller.Configure(cfg)
	writeJSON(w, 200, publicConfig(cfg))
}
func (s *server) groups(w http.ResponseWriter, r *http.Request) {
	cfg, err := readInput(w, r, s.controller.Config())
	if err != nil {
		writeError(w, 400, err)
		return
	}
	groups, err := NewMihomo(cfg).Groups(r.Context())
	if err != nil {
		writeError(w, 502, err)
		return
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	writeJSON(w, 200, map[string]any{"groups": groups, "message": "Clash API 连接成功"})
}
