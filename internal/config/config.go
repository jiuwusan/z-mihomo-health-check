package config

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

type Config struct {
	DingTalkWebhook             string `json:"dingtalk_webhook"`
	FeishuWebhook               string `json:"feishu_webhook"`
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
	for _, hook := range []string{c.DingTalkWebhook, c.FeishuWebhook} {
		if hook == "" {
			continue
		}
		u, err := url.Parse(hook)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" || len(hook) > 4096 || strings.IndexFunc(hook, unicode.IsControl) >= 0 {
			return errors.New("机器人 Webhook 须为有效的 HTTP/HTTPS 地址，不得含账户、片段或控制字符，且不能超过 4096 字节")
		}
	}
	u, err := url.Parse(c.APIURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("Mihomo API 地址须为不带账户、查询参数和片段的 HTTP/HTTPS 地址")
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
