package qqapi

import (
	"Plrx/lib/constant"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// 审计等待：audit_id -> 结果通道
var (
	auditMu     sync.Mutex
	auditWaiter = make(map[string]chan auditResult)
)

type auditResult struct {
	approved  bool
	messageID string
}

const auditTimeout = 30 * time.Second

// waitAudit 注册审计等待并阻塞直到 audit 事件或超时。
func (c *Client) waitAudit(auditID string) error {
	ch := make(chan auditResult, 1)
	auditMu.Lock()
	auditWaiter[auditID] = ch
	auditMu.Unlock()

	select {
	case r := <-ch:
		if !r.approved {
			return fmt.Errorf("消息审核未通过")
		}
		return nil
	case <-time.After(auditTimeout):
		return fmt.Errorf("消息审核等待超时")
	}
}

// ResolveAudit 由 MESSAGE_AUDIT_PASS/REJECT 事件调用，resolve 等待者。
func ResolveAudit(auditID, messageID string, approved bool) {
	auditMu.Lock()
	ch, ok := auditWaiter[auditID]
	if ok {
		delete(auditWaiter, auditID)
	}
	auditMu.Unlock()
	if ok {
		ch <- auditResult{approved: approved, messageID: messageID}
	}
}

// chunkedUpload 分片上传：先准备上传，再逐片直传 COS，最后完成。
func (c *Client) chunkedUpload(target constant.MessageOrigin, groupID, userID string, up MediaUpload, header map[string]string) (string, error) {
	var prepEndpoint, finishEndpoint string
	if target == constant.GroupMessage {
		prepEndpoint = fmt.Sprintf("%v/v2/groups/%v/upload_prepare", c.ProxyAPI, groupID)
		finishEndpoint = fmt.Sprintf("%v/v2/groups/%v/upload_part_finish", c.ProxyAPI, groupID)
	} else {
		prepEndpoint = fmt.Sprintf("%v/v2/users/%v/upload_prepare", c.ProxyAPI, userID)
		finishEndpoint = fmt.Sprintf("%v/v2/users/%v/upload_part_finish", c.ProxyAPI, userID)
	}

	md5hex := md5HexBytes(up.Data)
	sha1hex := sha1HexBytes(up.Data)
	type UploadPrepareRequest struct {
		FileType int    `json:"file_type"`
		FileName string `json:"file_name"`
		FileSize int    `json:"file_size"`
		MD5      string `json:"md5"`
		SHA1     string `json:"sha1"`
	}
	var prep struct {
		UploadID  string `json:"upload_id"`
		BlockSize string `json:"block_size"`
		Parts     []struct {
			Index        int    `json:"index"`
			PresignedURL string `json:"presigned_url"`
		} `json:"parts"`
	}
	err := c.Request.Post(prepEndpoint, UploadPrepareRequest{
		FileType: up.FileType, FileName: up.Filename, FileSize: len(up.Data),
		MD5: md5hex, SHA1: sha1hex,
	}, &prep, header)
	if err != nil {
		return "", fmt.Errorf("upload prepare: %w", err)
	}

	blockSize, _ := strconv.Atoi(prep.BlockSize)
	if blockSize <= 0 {
		blockSize = 1024 * 1024
	}
	for _, part := range prep.Parts {
		start := (part.Index - 1) * blockSize
		end := min(start+blockSize, len(up.Data))
		chunk := up.Data[start:end]
		if err := c.Request.Put(part.PresignedURL, chunk, nil, nil); err != nil {
			return "", fmt.Errorf("upload part %d: %w", part.Index, err)
		}
		type PartFinishRequest struct {
			UploadID  string `json:"upload_id"`
			PartIndex int    `json:"part_index"`
			BlockSize int    `json:"block_size"`
			MD5       string `json:"md5"`
		}
		if err := c.Request.Post(finishEndpoint, PartFinishRequest{
			UploadID: prep.UploadID, PartIndex: part.Index,
			BlockSize: len(chunk), MD5: md5HexBytes(chunk),
		}, nil, header); err != nil {
			return "", fmt.Errorf("upload part finish %d: %w", part.Index, err)
		}
	}

	// 分片上传完成后用 sendFile 发送
	type SendFileRequest struct {
		UploadID   string `json:"upload_id"`
		SrvSendMsg bool   `json:"srv_send_msg"`
	}
	var sendEndpoint string
	if target == constant.GroupMessage {
		sendEndpoint = fmt.Sprintf("%v/v2/groups/%v/files", c.ProxyAPI, groupID)
	} else {
		sendEndpoint = fmt.Sprintf("%v/v2/users/%v/files", c.ProxyAPI, userID)
	}
	var result mediaUploadResponse
	if err := c.Request.Post(sendEndpoint, SendFileRequest{UploadID: prep.UploadID}, &result, header); err != nil {
		return "", fmt.Errorf("send chunked upload: %w", err)
	}
	if result.FileInfo == "" {
		return "", fmt.Errorf("chunked upload returned empty file_info")
	}
	return result.FileInfo, nil
}

func md5HexBytes(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

func sha1HexBytes(b []byte) string {
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

// 供 base64 使用占位（避免 unused import 误伤）
var _ = base64.StdEncoding
