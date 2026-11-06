package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"z-mihomo-health-check/internal/domain"
)

type Instances interface {
	List() []domain.InstanceInfo
	Add(string) (domain.InstanceInfo, error)
	Rename(string, string) error
	Delete(string) error
	ServeInstance(string, http.ResponseWriter, *http.Request)
}

func NewInstancesServer(m Instances) http.Handler {
	mux := http.NewServeMux()
	writeFailure := func(w http.ResponseWriter, err error) {
		code := 500
		var failure *domain.InstanceError
		if errors.As(err, &failure) {
			code = failure.Code
		}
		WriteError(w, code, err)
	}
	readName := func(w http.ResponseWriter, r *http.Request) (string, error) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var input *struct {
			Name string `json:"name"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input == nil {
			return "", domain.InstanceFailure(400, "请提供有效的实例名称 JSON 对象")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return "", domain.InstanceFailure(400, "请求只能包含一个 JSON 对象")
		}
		return input.Name, nil
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/instances", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"instances": m.List()}) })
	mux.HandleFunc("POST /api/instances", func(w http.ResponseWriter, r *http.Request) {
		name, err := readName(w, r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		info, err := m.Add(name)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeJSON(w, 201, info)
	})
	mux.HandleFunc("PUT /api/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		name, err := readName(w, r)
		if err == nil {
			err = m.Rename(r.PathValue("id"), name)
		}
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("DELETE /api/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := m.Delete(r.PathValue("id")); err != nil {
			writeFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/instances/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		cloned := r.Clone(r.Context())
		cloned.URL.Path = "/api/" + r.PathValue("action")
		cloned.URL.RawPath = ""
		m.ServeInstance(r.PathValue("id"), w, cloned)
	})
	mux.Handle("/", panelAssets())
	return panelMiddleware(mux)
}
