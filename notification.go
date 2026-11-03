package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const notificationTimeout = 3 * time.Second

// Deliver outside the state lock, with a shared deadline for both channels.
// Do not include URLs, response bodies or transport errors in history: they may contain credentials.
func (c *Controller) notify(ctx context.Context, cfg Config, rev uint64, reason, previous, current string) {
	ctx, cancel := context.WithTimeout(ctx, notificationTimeout)
	defer cancel()
	message := fmt.Sprintf("z-mihomo-health-check\n%s\n目标分组：%s\n原节点：%s\n当前节点：%s\n时间：%s", reason, cfg.Group, previous, current, time.Now().Format(time.RFC3339))
	var wg sync.WaitGroup
	for _, channel := range []struct {
		name, url string
		payload   any
	}{
		{"钉钉", cfg.DingTalkWebhook, map[string]any{"msgtype": "text", "text": map[string]string{"content": message}}},
		{"飞书", cfg.FeishuWebhook, map[string]any{"msg_type": "text", "content": map[string]string{"text": message}}},
	} {
		if channel.url == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sendWebhook(ctx, channel.url, channel.payload) != nil && ctx.Err() != context.Canceled {
				c.update(rev, func(s *Status) {
					message := s.Message
					addEvent(s, "warning", channel.name+"机器人通知发送失败，请检查 Webhook、机器人安全设置或网络")
					s.Message = message
				})
			}
		}()
	}
	wg.Wait()
}

func sendWebhook(ctx context.Context, hook string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: notificationTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("webhook HTTP failure")
	}
	var result struct {
		ErrCode    *int `json:"errcode"`
		Code       *int `json:"code"`
		StatusCode *int `json:"StatusCode"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result) != nil {
		return errors.New("invalid webhook response")
	}
	if result.ErrCode == nil && result.Code == nil && result.StatusCode == nil {
		return errors.New("missing webhook result")
	}
	for _, code := range []*int{result.ErrCode, result.Code, result.StatusCode} {
		if code != nil && *code != 0 {
			return errors.New("webhook rejected message")
		}
	}
	return nil
}
