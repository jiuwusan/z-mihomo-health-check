package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

const notificationTimeout = 3 * time.Second

func SendWebhook(ctx context.Context, hook string, payload any) error {
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
