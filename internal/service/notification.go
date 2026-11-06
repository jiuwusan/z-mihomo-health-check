package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/gateway"
)

const notificationTimeout = 3 * time.Second
const notificationTitle = "Mihomo 节点状态变更"

type notificationKind string

const (
	notificationFailover    notificationKind = "故障转移"
	notificationRecovery    notificationKind = "节点恢复"
	notificationUnavailable notificationKind = "全节点故障"
)

// Names from Mihomo and the registry are text, never Markdown or mention tags.
func notificationText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\\", "\\\\", "*", "\\*", "_", "\\_", "~", "\\~", "`", "\\`", "[", "\\[", "]", "\\]", "\r", " ", "\n", " ").Replace(value)
}

// lark_md uses HTML entities for literal Markdown punctuation.
func feishuNotificationText(value string) string {
	var text strings.Builder
	for _, r := range value {
		if r == '\r' || r == '\n' {
			r = ' '
		}
		if strings.ContainsRune("&<>*~_`[]()\\#!+-/$\"':", r) {
			fmt.Fprintf(&text, "&#%d;", r)
		} else {
			text.WriteRune(r)
		}
	}
	return text.String()
}

// Deliver outside the state lock, with a shared deadline for both channels.
// Do not include URLs, response bodies or transport errors in history: they may contain credentials.
func (c *Controller) notify(ctx context.Context, cfg config.Config, rev uint64, kind notificationKind, previous, current string) {
	c.mu.Lock()
	name := c.instanceName
	active := c.rev == rev && ctx.Err() == nil
	c.mu.Unlock()
	if !active {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, notificationTimeout)
	defer cancel()
	icon, color := "🟠", "orange"
	switch kind {
	case notificationRecovery:
		icon, color = "🟢", "green"
	case notificationUnavailable:
		icon, color = "🔴", "red"
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formatMessage := func(escape func(string) string) string {
		return fmt.Sprintf("**名称：** %s\n\n**状态：** %s %s\n\n**分组：** %s\n\n**原节点：** %s\n\n**新节点：** %s\n\n**时间：** %s", escape(name), icon, kind, escape(cfg.Group), escape(previous), escape(current), timestamp)
	}
	card := map[string]any{
		"config":   map[string]bool{"wide_screen_mode": true},
		"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": notificationTitle}, "template": color},
		"elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": formatMessage(feishuNotificationText)}}},
	}
	var wg sync.WaitGroup
	for _, channel := range []struct {
		name, url string
		payload   any
	}{
		{"钉钉", cfg.DingTalkWebhook, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": notificationTitle, "text": "### " + notificationTitle + "\n\n" + formatMessage(notificationText)}}},
		{"飞书", cfg.FeishuWebhook, map[string]any{"msg_type": "interactive", "card": card}},
	} {
		if channel.url == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if gateway.SendWebhook(ctx, channel.url, channel.payload) != nil && ctx.Err() != context.Canceled {
				c.update(rev, func(s *domain.Status) {
					message := s.Message
					addEvent(s, "warning", channel.name+"机器人通知发送失败，请检查 Webhook、机器人安全设置或网络")
					s.Message = message
				})
			}
		}()
	}
	wg.Wait()
}
