package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	client     http.Client
	maxRetries int
}

func Init(timeout int) *Client {
	return &Client{
		client: http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		maxRetries: 3,
	}
}

func (c *Client) Get(url string, result any, headers map[string]string) error {
	return c.request(http.MethodGet, url, nil, result, headers)
}

func (c *Client) Post(url string, body any, result any, headers map[string]string) error {
	return c.request(http.MethodPost, url, body, result, headers)
}

func (c *Client) Put(url string, body any, result any, headers map[string]string) error {
	return c.request(http.MethodPut, url, body, result, headers)
}

func (c *Client) Patch(url string, body any, result any, headers map[string]string) error {
	return c.request(http.MethodPatch, url, body, result, headers)
}

func (c *Client) Delete(url string, body any, result any, headers map[string]string) error {
	return c.request(http.MethodDelete, url, body, result, headers)
}

func (c *Client) Head(url string, headers map[string]string) error {
	return c.request(http.MethodHead, url, nil, nil, headers)
}

func (c *Client) request(method, url string, body any, result any, headers map[string]string) error {
	bodyReader, err := encodeBody(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", method, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.do(req, result)
}

func encodeBody(body any) (io.Reader, error) {
	switch v := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return bytes.NewReader(v), nil
	case string:
		return bytes.NewReader([]byte(v)), nil
	case io.Reader:
		return v, nil
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		return bytes.NewReader(jsonBytes), nil
	}
}

func isRetryableStatus(code int) bool {
	return code == http.StatusRequestTimeout ||
		code == http.StatusTooManyRequests ||
		code >= 500
}

func (c *Client) do(req *http.Request, result any) error {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
	}

	maxAttempts := c.maxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	delays := []time.Duration{200 * time.Millisecond, 300 * time.Millisecond, 500 * time.Millisecond}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := delays[len(delays)-1]
			if attempt-1 < len(delays) {
				delay = delays[attempt-1]
			}
			time.Sleep(delay)
		}

		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyBytes)), nil
			}
		} else {
			req.Body = nil
			req.ContentLength = 0
			req.GetBody = nil
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
			if isRetryableStatus(resp.StatusCode) {
				continue
			}
			return lastErr
		}

		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("failed to decode response json: %w", err)
			}
		}
		return nil
	}

	return lastErr
}
