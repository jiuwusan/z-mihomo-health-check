package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/httpapi"
	"z-mihomo-health-check/internal/repository"
	"z-mihomo-health-check/internal/service"
)

type managedInstance struct {
	lifetime       context.Context
	cancelLifetime context.CancelFunc
	mu             sync.RWMutex
	controller     *service.Controller
	history        *service.HistoryStore
	handler        http.Handler
	stopped        bool
	saveCancel     context.CancelFunc
	runDone        chan struct{}
	saveDone       chan error
}

func (in *managedInstance) start() {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.stopped || in.runDone != nil {
		return
	}
	in.runDone = make(chan struct{})
	go func() { defer close(in.runDone); in.controller.Run(in.lifetime) }()
	saveCtx, saveCancel := context.WithCancel(context.Background())
	in.saveCancel = saveCancel
	in.saveDone = make(chan error, 1)
	go func() {
		ticker := time.NewTicker(service.HistoryFlushInterval)
		defer ticker.Stop()
		in.saveDone <- in.history.Run(saveCtx, ticker.C)
	}()
}
func (in *managedInstance) stop() {
	// Cancel slow draft API requests before waiting for the write lock. Keep
	// the lock below so a concurrent configuration save cannot resurrect tasks.
	in.cancelLifetime()
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.stopped {
		return
	}
	in.stopped = true
	in.controller.Stop()
	var err error
	if in.runDone != nil {
		<-in.runDone
		in.saveCancel()
		err = <-in.saveDone
	} else {
		err = in.history.Flush()
	}
	if err != nil {
		log.Printf("实例停止时保存历史失败：%v", err)
	}
}
func (in *managedInstance) serve(w http.ResponseWriter, r *http.Request) {
	in.mu.RLock()
	defer in.mu.RUnlock()
	if in.stopped || in.lifetime.Err() != nil {
		httpapi.WriteError(w, 410, errors.New("实例已停止或删除"))
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(in.lifetime, cancel)
	defer stop()
	defer cancel()
	in.handler.ServeHTTP(w, r.Clone(ctx))
}

type InstanceManager struct {
	mu              sync.RWMutex
	edit            sync.Mutex
	root            string
	pool            *service.SitePool
	items           map[string]*managedInstance
	order           []domain.InstanceInfo
	started, closed bool
}

func validInstanceID(id string) bool {
	if len(id) != 34 || !strings.HasPrefix(id, "i-") {
		return false
	}
	_, err := hex.DecodeString(id[2:])
	return err == nil && strings.ToLower(id) == id
}
func instanceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", domain.InstanceFailure(400, "实例名称须为 1–80 个字符，不能包含控制字符")
	}
	return name, nil
}

func NewInstanceManager(root string, sites []string) (*InstanceManager, error) {
	pool, err := service.OpenSitePool(sites, filepath.Join(root, "site-pool.json"))
	if err != nil {
		return nil, err
	}
	m := &InstanceManager{root: root, pool: pool, items: map[string]*managedInstance{}}
	registry, err := repository.LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	if registry == nil {
		_, err = m.Add("默认实例")
		return m, err
	}
	names := map[string]bool{}
	for _, info := range registry.Instances {
		name, nameErr := instanceName(info.Name)
		if !validInstanceID(info.ID) || m.items[info.ID] != nil || nameErr != nil || name != info.Name || names[strings.ToLower(name)] {
			for _, in := range m.items {
				in.controller.Stop()
			}
			return nil, errors.New("实例列表包含无效或重复的 ID/名称")
		}
		in, openErr := m.open(info)
		if openErr != nil {
			for _, previous := range m.items {
				previous.controller.Stop()
			}
			return nil, openErr
		}
		m.items[info.ID] = in
		names[strings.ToLower(name)] = true
	}
	m.order = registry.Instances
	return m, nil
}
func (m *InstanceManager) directory(id string) string {
	return filepath.Join(m.root, "instances", id)
}
func (m *InstanceManager) open(info domain.InstanceInfo) (*managedInstance, error) {
	id := info.ID
	file := filepath.Join(m.directory(id), "config.json")
	cfg, err := repository.LoadConfig(file)
	if err != nil {
		return nil, errors.New("实例 " + id + " 配置加载失败：" + err.Error())
	}
	c := service.NewControllerWithPool(m.pool)
	c.SetInstanceName(info.Name)
	c.Configure(cfg)
	h, err := service.OpenHistoryStore(c, filepath.Join(m.directory(id), "history.json"))
	if err != nil {
		c.Stop()
		return nil, errors.New("实例 " + id + " 历史加载失败：" + err.Error())
	}
	lifetime, cancel := context.WithCancel(context.Background())
	return &managedInstance{lifetime: lifetime, cancelLifetime: cancel, controller: c, history: h, handler: httpapi.NewServer(c, file)}, nil
}
func (m *InstanceManager) List() []domain.InstanceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.InstanceInfo{}, m.order...)
}
func (m *InstanceManager) Get(id string) (*managedInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, false
	}
	in, ok := m.items[id]
	return in, ok
}

// edit serializes mutations, startup and shutdown; mu protects the published registry.
func (m *InstanceManager) checkName(name, id string) (string, error) {
	if m.closed {
		return "", domain.InstanceFailure(503, "服务正在停止")
	}
	name, err := instanceName(name)
	if err != nil {
		return "", err
	}
	for _, info := range m.order {
		if info.ID != id && strings.EqualFold(info.Name, name) {
			return "", domain.InstanceFailure(400, "实例名称已存在")
		}
	}
	return name, nil
}
func (m *InstanceManager) Add(name string) (domain.InstanceInfo, error) {
	m.edit.Lock()
	defer m.edit.Unlock()
	name, err := m.checkName(name, "")
	if err != nil {
		return domain.InstanceInfo{}, err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return domain.InstanceInfo{}, domain.InstanceFailure(500, "无法创建实例 ID")
	}
	info := domain.InstanceInfo{ID: "i-" + hex.EncodeToString(random[:]), Name: name}
	if err = repository.SaveConfig(filepath.Join(m.directory(info.ID), "config.json"), config.DefaultConfig()); err != nil {
		return domain.InstanceInfo{}, domain.InstanceFailure(500, err.Error())
	}
	in, err := m.open(info)
	if err != nil {
		return domain.InstanceInfo{}, err
	}
	items := append(m.List(), info)
	if err = repository.SaveRegistry(m.root, items); err != nil {
		in.controller.Stop()
		return domain.InstanceInfo{}, err
	}
	m.mu.Lock()
	m.items[info.ID] = in
	m.order = items
	m.mu.Unlock()
	if m.started {
		in.start()
	}
	return info, nil
}
func (m *InstanceManager) Rename(id, name string) error {
	m.edit.Lock()
	defer m.edit.Unlock()
	name, err := m.checkName(name, id)
	if err != nil {
		return err
	}
	in, ok := m.Get(id)
	if !ok {
		return domain.InstanceFailure(404, "实例不存在")
	}
	items := m.List()
	for i := range items {
		if items[i].ID == id {
			items[i].Name = name
		}
	}
	if err = repository.SaveRegistry(m.root, items); err != nil {
		return err
	}
	m.mu.Lock()
	m.order = items
	in.controller.SetInstanceName(name)
	m.mu.Unlock()
	return nil
}
func (m *InstanceManager) Delete(id string) error {
	m.edit.Lock()
	defer m.edit.Unlock()
	in, ok := m.Get(id)
	if !ok {
		return domain.InstanceFailure(404, "实例不存在")
	}
	items := m.List()
	if len(items) <= 1 {
		return domain.InstanceFailure(400, "至少保留一个实例，可通过配置停用探查")
	}
	kept := []domain.InstanceInfo{}
	for _, info := range items {
		if info.ID != id {
			kept = append(kept, info)
		}
	}
	if err := repository.SaveRegistry(m.root, kept); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.items, id)
	m.order = kept
	m.mu.Unlock()
	in.stop()
	return nil
}
func (m *InstanceManager) Start() {
	m.edit.Lock()
	defer m.edit.Unlock()
	if m.closed || m.started {
		return
	}
	m.started = true
	for _, info := range m.List() {
		in, _ := m.Get(info.ID)
		in.start()
	}
}
func (m *InstanceManager) Close() {
	m.edit.Lock()
	defer m.edit.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	items := make([]*managedInstance, 0, len(m.items))
	for _, in := range m.items {
		items = append(items, in)
	}
	m.mu.Unlock()
	var done sync.WaitGroup
	for _, in := range items {
		done.Add(1)
		go func() { defer done.Done(); in.stop() }()
	}
	done.Wait()
}

func (m *InstanceManager) ServeInstance(id string, w http.ResponseWriter, r *http.Request) {
	in, ok := m.Get(id)
	if !ok {
		httpapi.WriteError(w, 404, errors.New("实例不存在"))
		return
	}
	in.serve(w, r)
}
