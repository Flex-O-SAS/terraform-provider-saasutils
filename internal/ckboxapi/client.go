// Copyright (c) HashiCorp, Inc.

package ckboxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"io"
	"net/http"
	"sync"
	"time"
)

type APIClient struct {
	baseURL        string
	http           *http.Client
	defaultHeaders map[string]string
	mutex          sync.Mutex
}

func NewCkboxClient(baseURL string, timeout time.Duration) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: timeout,
		},
		defaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
	}
}

func (c *APIClient) SetHeader(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.defaultHeaders[key] = value
}

func (c *APIClient) UnsetHeader(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c == nil || c.defaultHeaders == nil {
		return
	}
	delete(c.defaultHeaders, key)
}

func (c *APIClient) GetHeader(key string) (string, bool) {
	if c.defaultHeaders == nil {
		return "", false
	}
	v, ok := c.defaultHeaders[key]
	return v, ok
}

// Do envoie une requête HTTP et retourne le body brut + status code.
func (c *APIClient) Do(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, int, error) {

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	var bodyLog any = nil
	if body != nil {
		bodyLog = string(body)
	}

	tflog.Debug(ctx, "HTTP request", map[string]any{
		"method": method,
		"url":    c.baseURL + path,
		"body":   bodyLog,
	})
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

func (c *APIClient) CallInto(
	ctx context.Context,
	method, path string,
	payload any,
	out any,
) (int, error) {

	var body []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = b
	}

	respBody, status, err := c.Do(ctx, method, path, body)
	if err != nil {
		return status, err
	}

	if out == nil || len(respBody) == 0 {
		return status, nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return status, fmt.Errorf("decode response: %w (body=%s)", err, string(respBody))
	}

	return status, nil
}
