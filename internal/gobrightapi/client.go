package gobrightapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type APIClient struct {
	baseURL          string
	http             *http.Client
	defaultHeaders   map[string]string
	mutex            sync.Mutex
	organizationCode string
}

func NewClient(baseURL, organizationCode string, timeout time.Duration) *APIClient {
	return &APIClient{
		baseURL:          baseURL,
		organizationCode: organizationCode,
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

func (c *APIClient) GetHeader(key string) (string, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.defaultHeaders == nil {
		return "", false
	}
	v, ok := c.defaultHeaders[key]
	return v, ok
}

func (c *APIClient) GetOrganizationCode() string {
	return c.organizationCode
}

// Do performs a JSON-style request using the current defaultHeaders.
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

	url := c.baseURL + path
	tflog.Debug(ctx, "GoBright HTTP request", map[string]any{
		"method": method,
		"url":    url,
		"body":   bodyLog,
	})
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}

	c.mutex.Lock()
	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}
	c.mutex.Unlock()

	resp, err := c.http.Do(req)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP transport error", map[string]any{
			"method": method,
			"url":    url,
			"error":  err.Error(),
		})
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP response read error", map[string]any{
			"method": method,
			"url":    url,
			"status": resp.StatusCode,
			"error":  err.Error(),
		})
		return nil, resp.StatusCode, err
	}

	tflog.Debug(ctx, "GoBright HTTP response", map[string]any{
		"method": method,
		"url":    url,
		"status": resp.StatusCode,
		"body":   string(respBody),
	})

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// CallInto marshals payload to JSON, performs the request, and unmarshals the
// response body into out (if non-nil).
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

// doForm performs a single request with the given override headers replacing
// the default headers (used for the auth flow), encoding a url.Values body.
func (c *APIClient) doForm(
	ctx context.Context,
	method, path string,
	headers map[string]string,
	form url.Values,
) ([]byte, int, error) {

	body := form.Encode()
	reader := strings.NewReader(body)

	url := c.baseURL + path
	tflog.Debug(ctx, "GoBright HTTP form request", map[string]any{
		"method": method,
		"url":    url,
		"body":   body,
	})
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP form transport error", map[string]any{
			"method": method,
			"url":    url,
			"error":  err.Error(),
		})
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP form response read error", map[string]any{
			"method": method,
			"url":    url,
			"status": resp.StatusCode,
			"error":  err.Error(),
		})
		return nil, resp.StatusCode, err
	}

	tflog.Debug(ctx, "GoBright HTTP form response", map[string]any{
		"method": method,
		"url":    url,
		"status": resp.StatusCode,
		"body":   string(respBody),
	})

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// doWithHeaders is like Do but uses an explicit headers map (override) for the
// request, leaving the client's default headers untouched. Used for the
// /api/users/login call during auth.
func (c *APIClient) doWithHeaders(
	ctx context.Context,
	method, path string,
	headers map[string]string,
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

	url := c.baseURL + path
	tflog.Debug(ctx, "GoBright HTTP request (override headers)", map[string]any{
		"method": method,
		"url":    url,
		"body":   bodyLog,
	})
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP (override headers) transport error", map[string]any{
			"method": method,
			"url":    url,
			"error":  err.Error(),
		})
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		tflog.Debug(ctx, "GoBright HTTP (override headers) response read error", map[string]any{
			"method": method,
			"url":    url,
			"status": resp.StatusCode,
			"error":  err.Error(),
		})
		return nil, resp.StatusCode, err
	}

	tflog.Debug(ctx, "GoBright HTTP response (override headers)", map[string]any{
		"method": method,
		"url":    url,
		"status": resp.StatusCode,
		"body":   string(respBody),
	})

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}
