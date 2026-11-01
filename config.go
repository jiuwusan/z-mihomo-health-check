package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type Config struct {
	APIURL                      string `json:"api_url"`
	Secret                      string `json:"secret"`
	Group                       string `json:"group"`
	IntervalSeconds             int    `json:"interval_seconds"`
	FailureThreshold            int    `json:"failure_threshold"`
	TimeoutSeconds              int    `json:"timeout_seconds"`
	Enabled                     bool   `json:"enabled"`
	PersistHistory              bool   `json:"persist_history"`
	RecoveryEnabled             bool   `json:"recovery_enabled"`
	RecoveryInitialDelaySeconds int    `json:"recovery_initial_delay_seconds"`
	RecoveryIntervalSeconds     int    `json:"recovery_interval_seconds"`
	RecoverySuccessThreshold    int    `json:"recovery_success_threshold"`
}

func DefaultConfig() Config {
	return Config{APIURL: "http://mihomo:9090", Group: "🚀 节点选择", IntervalSeconds: 5, FailureThreshold: 1, TimeoutSeconds: 4, RecoveryInitialDelaySeconds: 300, RecoveryIntervalSeconds: 60, RecoverySuccessThreshold: 3}
}
func (c Config) Validate() error {
	u, err := url.Parse(c.APIURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("Clash API 地址须为不带账户、查询参数和片段的 HTTP/HTTPS 地址")
	}
	if strings.TrimSpace(c.Group) == "" || len(c.Group) > 512 {
		return errors.New("目标分组不能为空且不能超过 512 字节")
	}
	if c.IntervalSeconds < 1 || c.IntervalSeconds > 86400 {
		return errors.New("探查间隔范围为 1–86400 秒")
	}
	// Mihomo's single-proxy delay endpoint parses timeout as signed int16 ms.
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 32 {
		return errors.New("超时范围为 1–32 秒（兼容 Mihomo delay 接口上限）")
	}
	if c.FailureThreshold < 1 || c.FailureThreshold > 100 {
		return errors.New("连续失败次数范围为 1–100")
	}
	if c.RecoveryInitialDelaySeconds < 0 || c.RecoveryInitialDelaySeconds > 86400 {
		return errors.New("首次最低间隔范围为 0–86400 秒")
	}
	if c.RecoveryIntervalSeconds < 1 || c.RecoveryIntervalSeconds > 86400 {
		return errors.New("恢复探查间隔范围为 1–86400 秒")
	}
	if c.RecoverySuccessThreshold < 1 || c.RecoverySuccessThreshold > 100 {
		return errors.New("连续恢复成功次数范围为 1–100")
	}
	if len(c.Secret) > 4096 || strings.IndexFunc(c.Secret, unicode.IsControl) >= 0 {
		return errors.New("Secret 不能包含控制字符且不能超过 4096 字节")
	}
	return nil
}
func LoadConfig(filename string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, errors.New("无法读取配置文件")
	}
	if json.Unmarshal(b, &c) != nil {
		return c, errors.New("配置文件不是有效 JSON")
	}
	return c, c.Validate()
}
func SaveConfig(filename string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return errors.New("无法创建数据目录")
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(filename), ".config-*")
	if err != nil {
		return errors.New("无法创建配置临时文件")
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("写入配置失败")
	}
	if os.Rename(name, filename) != nil {
		return errors.New("替换配置文件失败")
	}
	return nil
}
