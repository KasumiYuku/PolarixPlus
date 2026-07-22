package qqapi

import (
	"Plrx/lib/requests"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	ProxyAPI    string
	AppID       string
	AppSecret   string
	Request     *requests.Client
	accessToken string
	expireAt    time.Time
	lock        sync.RWMutex
}

func Init(AppID string, AppSecret string, ProxyAPI string, requests *requests.Client) Client {
	return Client{
		AppID:       AppID,
		AppSecret:   AppSecret,
		ProxyAPI:    ProxyAPI,
		Request:     requests,
		accessToken: "",
		lock:        sync.RWMutex{},
	}
}

// 生成Access Token
func (c *Client) getAccessToken() (string, error) {
	// log.Print("正在生成AccessToken")
	c.lock.RLock()
	if c.accessToken != "" && time.Now().Before(c.expireAt) {
		// 有效
		// log.Print("已有AccessToken且未过期, 返回")
		c.lock.RUnlock()
		return c.accessToken, nil
	}
	c.lock.RUnlock() // 释放之前的读取锁
	c.lock.Lock()    // 获取写入锁
	defer c.lock.Unlock()
	// log.Print("重新申请AccessToken")
	// 再次检查
	if c.accessToken != "" && time.Now().Before(c.expireAt) {
		return c.accessToken, nil
	}
	// 获取新的Token
	initData := fmt.Appendf(nil, `{"appId":"%s", "clientSecret":"%s"}`, c.AppID, c.AppSecret)
	// 数据模型
	type TokenData struct {
		AccessToken string `json:"access_token"`
		ExpireTime  string `json:"expires_in"`
	}
	var tokenData TokenData
	err := c.Request.Post("https://bots.qq.com/app/getAppAccessToken", initData, &tokenData, make(map[string]string))
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
	// log.Printf("生成AccessToken完毕: %v", c.accessToken)
	return tokenData.AccessToken, nil
}

func (c *Client) generateHeader() (map[string]string, error) {
	var result map[string]string = make(map[string]string)
	token, err := c.getAccessToken()
	if err != nil {
		return map[string]string{}, err
	}
	result["Authorization"] = fmt.Sprintf("QQBot %v", token)
	result["Content-Type"] = "application/json"
	return result, nil
}

// 发送群消息
func (c *Client) SendGroupMessage(data []byte, groupId string) error {
	// 获取请求头
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	// log.Printf("发送给%v, data = %v\n", fmt.Sprintf("%v/v2/groups/%v/messages", c.ProxyAPI, groupId), string(data))
	err = c.Request.Post(fmt.Sprintf("%v/v2/groups/%v/messages", c.ProxyAPI, groupId), data, nil, header)
	if err != nil {
		return err
	}
	return nil
}

// 发送私信消息
func (c *Client) SendPrivateMessage(data []byte, userId string) error {
	// 获取请求头
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	err = c.Request.Post(fmt.Sprintf("%v/v2/users/%v/messages", c.ProxyAPI, userId), data, nil, header)
	if err != nil {
		return err
	}
	return nil
}

// 回复回调按钮
func (c *Client) InteracteCallback(eventId string) error {
	header, err := c.generateHeader()
	if err != nil {
		return err
	}
	return c.Request.Put(fmt.Sprintf("%v/interactions/%v", c.ProxyAPI, eventId), nil, nil, header)
}
