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
	client http.Client
}

func Init(timeout int) *Client {
	return &Client{
		client: http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (c *Client) Put(url string, result any, headers map[string]string) error {
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}

	// 写入自定义 Header
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.do(req, result)
}

func (c *Client) Get(url string, result any, headers map[string]string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	// 写入自定义 Header
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.do(req, result)
}

func (c *Client) Post(url string, body any, result any, headers map[string]string) error {
	var bodyReader io.Reader

	switch v := body.(type) {
	case []byte:
		bodyReader = bytes.NewReader(v)
	case nil:
		bodyReader = nil
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal post body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	// 如果有请求体，自动加上Content-Type
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 写入自定义Header
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.do(req, result)
}

// 抽取公共的请求执行与解析逻辑
func (c *Client) do(req *http.Request, result any) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 校验 HTTP 状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 读取 Body 内容用于错误信息
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 如果传入了接收结果的变量，则解析 JSON
	if result != nil {
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(result); err != nil {
			return fmt.Errorf("failed to decode response json: %w", err)
		}
	}

	return nil
}
