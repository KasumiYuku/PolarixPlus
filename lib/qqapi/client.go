package qqapi

import (
	"Plrx/lib/assets"
	"Plrx/lib/constant"
	"Plrx/lib/requests"
	"Plrx/lib/state"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

type mediaUploadResponse struct {
	FileInfo string `json:"file_info"`
}

const defaultTokenAPI = "https://bots.qq.com/app/getAppAccessToken"

// Client QQ API 客户端。
type Client struct {
	ProxyAPI            string
	AppID               string
	AppSecret           string
	Request             *requests.Client
	Assets              *assets.ImageHost
	GlobalMarkdown      bool
	MarkdownVerifyImage bool
	RetryWhen           []int
	UploadThreshold     int // 超过该字节数用分片上传
	TokenAPI            string

	tokenReq    *requests.Client // token 签发通道, 独立超时
	accessToken string
	expireAt    time.Time
	lock        sync.RWMutex
}

func Init(AppID string, AppSecret string, ProxyAPI string, req *requests.Client) Client {
	return Client{
		AppID:     AppID,
		AppSecret: AppSecret,
		ProxyAPI:  ProxyAPI,
		Request:   req,
		TokenAPI:  defaultTokenAPI,
		tokenReq:  requests.Init(20),
		lock:      sync.RWMutex{},
	}
}

// SetAssets 注入图床聚合器。
func (c *Client) SetAssets(h *assets.ImageHost) { c.Assets = h }

// SetMessageOptions 注入消息管道配置。
func (c *Client) SetMessageOptions(globalMarkdown, markdownVerifyImage bool, retryWhen []int, uploadThreshold int) {
	c.GlobalMarkdown = globalMarkdown
	c.MarkdownVerifyImage = markdownVerifyImage
	c.RetryWhen = retryWhen
	c.UploadThreshold = uploadThreshold
}

// AccessToken 返回当前有效的 access token（供外部使用，如 WebSocket 鉴权）。
func (c *Client) AccessToken() (string, error) {
	return c.getAccessToken()
}

// InvalidateToken 清空缓存的 access token, 下次调用强制重取。
func (c *Client) InvalidateToken() {
	c.lock.Lock()
	c.accessToken = ""
	c.expireAt = time.Time{}
	c.lock.Unlock()
}

// GatewayBotInfo /gateway/bot 响应：网关地址 + 会话启动限额。
type GatewayBotInfo struct {
	URL    string `json:"url"`
	Shards int    `json:"shards"`
	Limit  struct {
		Total         int64 `json:"total"`
		Remaining     int64 `json:"remaining"`
		ResetAfter    int64 `json:"reset_after"`     // 毫秒
		MaxConcurrent int   `json:"max_concurrency"` // 建议的并发连接数
	} `json:"session_start_limit"`
}

// GatewayBot 获取网关地址与会话启动限额。remaining 耗尽时需等待 reset_after 再建连。
func (c *Client) GatewayBot() (*GatewayBotInfo, error) {
	header, err := c.generateHeader()
	if err != nil {
		return nil, err
	}
	var result GatewayBotInfo
	if err := c.Request.Get(fmt.Sprintf("%v/gateway/bot", c.ProxyAPI), &result, header); err != nil {
		return nil, fmt.Errorf("get gateway bot: %w", err)
	}
	if result.URL == "" {
		return nil, fmt.Errorf("gateway bot url is empty")
	}
	return &result, nil
}

// GatewayURL 获取网关 WebSocket 地址。
func (c *Client) GatewayURL() (string, error) {
	header, err := c.generateHeader()
	if err != nil {
		return "", err
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := c.Request.Get(fmt.Sprintf("%v/gateway", c.ProxyAPI), &result, header); err != nil {
		return "", fmt.Errorf("get gateway url: %w", err)
	}
	if result.URL == "" {
		return "", fmt.Errorf("gateway url is empty")
	}
	return result.URL, nil
}

// 生成Access Token
func (c *Client) getAccessToken() (string, error) {
	c.lock.RLock()
	if c.accessToken != "" && time.Now().Before(c.expireAt) {
		token := c.accessToken
		c.lock.RUnlock()
		return token, nil
	}
	c.lock.RUnlock()
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.accessToken != "" && time.Now().Before(c.expireAt) {
		return c.accessToken, nil
	}
	initData := fmt.Appendf(nil, `{"appId":"%s", "clientSecret":"%s"}`, c.AppID, c.AppSecret)
	type TokenData struct {
		AccessToken string `json:"access_token"`
		ExpireTime  string `json:"expires_in"`
	}
	var tokenData TokenData
	err := c.tokenReq.Post(c.TokenAPI, initData, &tokenData, make(map[string]string))
	if err != nil {
		return "", err
	}
	if tokenData.AccessToken == "" {
		return "", fmt.Errorf("Access token in data is null")
	}
	expriedTime, err := strconv.Atoi(tokenData.ExpireTime)
	if err != nil {
		return "", fmt.Errorf("Incorrect experie_time")
	}
	buffer := 50 // 还有50秒的时候就刷新
	remaining := time.Duration(expriedTime) * time.Second
	c.expireAt = time.Now().Add(remaining - (time.Duration(buffer) * time.Second))
	c.accessToken = tokenData.AccessToken
	return tokenData.AccessToken, nil
}

func (c *Client) generateHeader() (map[string]string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return map[string]string{}, err
	}
	return map[string]string{
		"Authorization": fmt.Sprintf("QQBot %v", token),
		"Content-Type":  "application/json",
	}, nil
}

// SendMessageResult 发送结果。
type SendMessageResult struct {
	ID        string `json:"id"`
	AuditID   string `json:"audit_id"`
	Timestamp string `json:"timestamp"`
}

// sendWithRetry 发送并处理业务错误码重试与审计等待。
func (c *Client) sendWithRetry(endpoint string, data []byte) (*SendMessageResult, error) {
	header, err := c.generateHeader()
	if err != nil {
		return nil, err
	}
	var result SendMessageResult
	for attempt := 0; ; attempt++ {
		raw, err := c.Request.DoBytes("POST", endpoint, requests.Bytes(data), header)
		if err != nil {
			// 透传 HTTP 层错误
			return nil, err
		}
		if len(raw) == 0 {
			return &result, nil
		}
		if err := json.Unmarshal(raw, &result); err == nil && result.ID != "" {
			return &result, nil
		}
		// 解析业务错误码
		var errResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				MessageAudit *struct {
					AuditID string `json:"audit_id"`
				} `json:"message_audit"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &errResp); err == nil && errResp.Code != 0 {
			// 审计等待：注册等待，直到 audit 事件
			if errResp.Code == 304023 && errResp.Data.MessageAudit != nil {
				if err := c.waitAudit(errResp.Data.MessageAudit.AuditID); err != nil {
					return nil, err
				}
				return &result, nil
			}
			// 业务错误码重试
			if IsRetryable(errResp.Code, c.RetryWhen) && attempt < 2 {
				time.Sleep(time.Duration(300*(attempt+1)) * time.Millisecond)
				continue
			}
			return nil, &QQError{Code: errResp.Code, Message: errResp.Message}
		}
		return &result, nil
	}
}

// 发送群消息
func (c *Client) SendGroupMessage(data []byte, groupId string) error {
	_, err := c.sendWithRetry(fmt.Sprintf("%v/v2/groups/%v/messages", c.ProxyAPI, groupId), data)
	if err == nil {
		state.IncSent()
	}
	return err
}

// 发送私信消息
func (c *Client) SendPrivateMessage(data []byte, userId string) error {
	_, err := c.sendWithRetry(fmt.Sprintf("%v/v2/users/%v/messages", c.ProxyAPI, userId), data)
	if err == nil {
		state.IncSent()
	}
	return err
}

// UploadImage 上传本地图片到 QQ，返回 file_info。
func (c *Client) UploadImage(target constant.MessageOrigin, groupID, userID, filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read image for upload: %w", err)
	}
	return c.UploadMedia(target, groupID, userID, MediaUpload{FileType: 1, Data: data, Filename: filePath})
}

// MediaUpload 媒体上传参数。
type MediaUpload struct {
	FileType int    // 1 图片 2 视频 3 音频 4 文件
	Data     []byte // 文件内容
	Filename string
	URL      string // 公网 URL 直传（与 Data 二选一）
}

// UploadMedia 上传媒体到 QQ，返回 file_info。超过阈值自动分片上传。
func (c *Client) UploadMedia(target constant.MessageOrigin, groupID, userID string, up MediaUpload) (string, error) {
	header, err := c.generateHeader()
	if err != nil {
		return "", err
	}
	threshold := c.UploadThreshold
	if threshold <= 0 {
		threshold = 3 << 20
	}

	var endpoint string
	switch target {
	case constant.GroupMessage:
		endpoint = fmt.Sprintf("%v/v2/groups/%v/files", c.ProxyAPI, groupID)
	case constant.PrivateMessage:
		endpoint = fmt.Sprintf("%v/v2/users/%v/files", c.ProxyAPI, userID)
	default:
		return "", fmt.Errorf("unknown message target type: %v", target)
	}

	// 大文件分片上传
	if len(up.Data) > threshold && len(up.Data) > 0 {
		return c.chunkedUpload(target, groupID, userID, up, header)
	}

	body := map[string]any{
		"file_type":    up.FileType,
		"srv_send_msg": false,
	}
	if up.URL != "" {
		body["url"] = up.URL
	} else if len(up.Data) > 0 {
		body["file_data"] = base64.StdEncoding.EncodeToString(up.Data)
	} else {
		return "", fmt.Errorf("upload media: both url and data empty")
	}
	if up.Filename != "" {
		body["file_name"] = up.Filename
	}

	var result mediaUploadResponse
	if err := c.Request.Post(endpoint, body, &result, header); err != nil {
		return "", fmt.Errorf("upload media to QQ: %w", err)
	}
	if result.FileInfo == "" {
		return "", fmt.Errorf("QQ media upload returned empty file_info")
	}
	return result.FileInfo, nil
}

// InteracteCallback 回执按钮交互，终止 QQ 端按钮 loading。
// code=0 表示正常处理完成；QQ 要求 3 秒内回执，超时按钮会判定失败。
func (c *Client) InteracteCallback(eventId string) error {
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	return c.Request.Put(fmt.Sprintf("%v/interactions/%v", c.ProxyAPI, eventId), map[string]int{"code": 0}, nil, header)
}

// 同意入群请求
func (c *Client) AcceptGroupJoinRequest(requestId, groupId, userId string) error {
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	type Data struct {
		Op  string `json:"op"`
		Rid string `json:"join_request_id"`
	}
	return c.Request.Post(fmt.Sprintf("%v/v2/groups/%v/approval_join_request/%v", c.ProxyAPI, groupId, userId), Data{Op: "approve", Rid: requestId}, nil, header)
}

// 拒绝入群请求
func (c *Client) RejectGroupJoinRequest(requestId, groupId, userId, reason string) error {
	return c.approveGroupJoinRequest(requestId, groupId, userId, reason, false)
}

// 拒绝入群请求并拉黑
func (c *Client) RejectGroupJoinRequestAndAddToBlacklist(requestId, groupId, userId, reason string) error {
	return c.approveGroupJoinRequest(requestId, groupId, userId, reason, true)
}

func (c *Client) approveGroupJoinRequest(requestId, groupId, userId, reason string, blacklist bool) error {
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	type Data struct {
		Op     string `json:"op"`
		Rid    string `json:"join_request_id"`
		Reason string `json:"reject_reason,omitempty"`
		A      bool   `json:"add_to_member_blacklist,omitempty"`
	}
	data := Data{Op: "decline", Rid: requestId, Reason: reason, A: blacklist}
	return c.Request.Post(fmt.Sprintf("%v/v2/groups/%v/approval_join_request/%v", c.ProxyAPI, groupId, userId), data, nil, header)
}
