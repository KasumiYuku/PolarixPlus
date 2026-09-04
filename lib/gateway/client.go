package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"sync"
	"time"

	"Plrx/lib/constant"
	"Plrx/lib/logx"
	"Plrx/lib/middleware"
	"Plrx/lib/qqapi"
	"Plrx/lib/structers"

	"github.com/gorilla/websocket"
)

var logger = logx.New("gateway")

const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opResume         = 6
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11
)

// 重连原因分类哨兵：作用域内根据类别决定是快速会话恢复还是限量等待后重新鉴权。
var (
	errInvalidSession = errors.New("会话失效，需重新鉴权")
	errReconnect      = errors.New("服务端要求重连")
)

type frame struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	T  string          `json:"t,omitempty"`
	S  int64           `json:"s,omitempty"`
	ID string          `json:"id,omitempty"`
}

// Client WebSocket 网关客户端。
type Client struct {
	api        *qqapi.Client
	gatewayURL string
	intents    int
	shard      [2]int

	mu        sync.Mutex
	conn      *websocket.Conn
	sessionID string
	seq       int64
	connAt    time.Time // 当前连接建立时刻, 供状态页展示
	ackAt     time.Time // 最近一次心跳 ACK 时刻

	ping *time.Ticker
	// acked 心跳护栏：上一次心跳未获 ACK 视为连接假死，主动断开触发重连。
	acked bool

	stopOnce sync.Once
	stop     chan struct{}
	stopped  chan struct{}
}

// New 创建网关客户端。
func New(api *qqapi.Client, gatewayURL string, intents int, shard [2]int) *Client {
	if shard[0] == 0 && shard[1] == 0 {
		shard = [2]int{0, 1}
	}
	return &Client{
		api:        api,
		gatewayURL: gatewayURL,
		intents:    intents,
		shard:      shard,
		stop:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

// Start 启动网关并阻塞直到进程退出（调用方应 go Start）。
// 按错误类别决定后续策略：网络抖动走指数退避快速恢复；
// opReconnect 保留会话走 RESUME 快速续连；会话失效走限额感知重新鉴权。
// backoff 为跨重连周期持续的状态：瞬时失败逐次翻倍，恢复后归零，避免频率冲突。
func (c *Client) Start() {
	defer close(c.stopped)
	backoff := time.Second
	for {
		if c.closed() {
			return
		}
		err := c.run()
		switch {
		case err == nil:
			// 理论上 run 不会正常返回；防御性兜底
			if !c.breathe(time.Second) {
				return
			}
		case errors.Is(err, errReconnect):
			// 服务端要求重连：保留会话，短暂退避后 RESUME（续上新连接）
			// 链路明确可用，重置退避阶梯，避免网络历史把新连接的重连拖慢。
			backoff = time.Second
			if !c.breathe(jitter(time.Second)) {
				return
			}
		case errors.Is(err, errInvalidSession):
			// 会话被服务端丢弃：放弃旧会话，按启动限额决定何时重新鉴权
			c.dropSession()
			if !c.waitForSlot() {
				return
			}
		default:
			// 网络层错误：指数退避（1s→30s）+ jitter 防惊群
			delay := jitter(backoff)
			logger.Warnf("网络断开重连：%v，%.0fs 后重试", err, delay.Seconds())
			if !c.breathe(delay) {
				return
			}
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

// Stop 停止网关并清理资源。幂等，可被并发调用。
func (c *Client) Stop() {
	c.stopOnce.Do(func() { close(c.stop) })
	c.closeConn()
	<-c.stopped
}

// Status 网关实时状态, 管理台轮询用。
type Status struct {
	Connected bool   `json:"connected"`
	SessionID string `json:"session_id"`
	Seq       int64  `json:"seq"`
	Heartbeat bool   `json:"heartbeat_ack"` // 上一次心跳是否已获 ACK
	SinceMS   int64  `json:"since_ms"`      // 当前连接已保持时长
}

func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	since := int64(0)
	if !c.connAt.IsZero() {
		since = time.Since(c.connAt).Milliseconds()
	}
	return Status{
		Connected: c.conn != nil,
		SessionID: c.sessionID,
		Seq:       c.seq,
		Heartbeat: c.acked,
		SinceMS:   since,
	}
}

func (c *Client) closed() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

// breathe 在退避间隔与停止信号之间做选择；返回 false 表示应退出主循环。
func (c *Client) breathe(d time.Duration) bool {
	if d <= 0 {
		return !c.closed()
	}
	select {
	case <-c.stop:
		return false
	case <-time.After(d):
		return !c.closed()
	}
}

// jitter 在 d 基础上叠加 ±25% 随机抖动，避免多实例断线后同步重连（thundering herd）。
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	spread := int64(d) / 2
	return d + time.Duration(rand.Int64N(spread)-spread/2)
}

func (c *Client) run() error {
	token, err := c.api.AccessToken()
	if err != nil {
		return fmt.Errorf("获取 access token: %w", err)
	}
	u, err := url.Parse(c.gatewayURL)
	if err != nil {
		return fmt.Errorf("解析网关地址: %w", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("连接网关 %s: %w", u.Redacted(), err)
	}
	c.setConn(conn)
	c.mu.Lock()
	c.connAt = time.Now()
	c.mu.Unlock()
	defer c.closeConn()
	// 每个网络帧都要更新读超时，防止服务端异常静默导致读卡死。
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))

	if err := c.handshake(conn, token); err != nil {
		return err
	}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var f frame
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if err := c.handle(conn, f, token); err != nil {
			return err
		}
	}
}

func (c *Client) handshake(conn *websocket.Conn, token string) error {
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("握手读帧: %w", err)
		}
		var f frame
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if f.Op != opHello {
			continue
		}
		var hello struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		}
		json.Unmarshal(f.D, &hello)
		if hello.HeartbeatInterval > 0 {
			c.startHeartbeat(conn, hello.HeartbeatInterval)
		}
		payload := map[string]any{"token": "QQBot " + token}
		if sessID := c.getSession(); sessID != "" {
			// 有存活会话：走 RESUME，续上 seq，避免重新鉴权与事件重放。
			payload["session_id"] = sessID
			payload["seq"] = c.getSeq()
			conn.WriteJSON(frame{Op: opResume, D: mustJSON(payload)})
		} else {
			payload["intents"] = c.intents
			payload["shard"] = c.shard
			conn.WriteJSON(frame{Op: opIdentify, D: mustJSON(payload)})
		}
		return nil
	}
}

func (c *Client) startHeartbeat(conn *websocket.Conn, interval int) {
	c.mu.Lock()
	if c.ping != nil {
		c.ping.Stop()
	}
	c.acked = true
	c.ping = time.NewTicker(time.Duration(interval) * time.Millisecond)
	ping := c.ping
	c.mu.Unlock()
	go func() {
		for range ping.C {
			c.mu.Lock()
			healthy := c.acked
			c.acked = false
			c.mu.Unlock()
			if !healthy {
				// 上次心跳未 ACK：连接假死，掐断触发重连。
				c.closeConn()
				return
			}
			conn.WriteJSON(frame{Op: opHeartbeat, D: mustJSON(map[string]any{"seq": c.getSeq()})})
		}
	}()
}

func (c *Client) handle(conn *websocket.Conn, f frame, token string) error {
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	switch f.Op {
	case opHeartbeatACK:
		c.mu.Lock()
		c.acked = true
		c.ackAt = time.Now()
		c.mu.Unlock()
	case opReconnect:
		return errReconnect
	case opInvalidSession:
		// d=false 表示会话可恢复，重连时走 RESUME；d=true 表明会话已被服务端丢弃，必须重新鉴权。
		if !bytesEq(f.D, false) {
			return errInvalidSession
		}
		return fmt.Errorf("%w：可恢复会话，重连时 RESUME", errReconnect)
	case opDispatch:
		if f.T == "READY" {
			var ready struct {
				SessionID string `json:"session_id"`
			}
			json.Unmarshal(f.D, &ready)
			c.setSession(ready.SessionID)
			logger.Infof("连接就绪，session=%s", ready.SessionID)
		}
		if f.T == "RESUMED" {
			logger.Infof("会话恢复成功")
		}
		if f.S > c.getSeq() {
			c.mu.Lock()
			c.seq = f.S
			c.mu.Unlock()
		}
		if !constant.IsValidEventType(f.T) {
			return nil
		}
		c.dispatch(f)
	}
	return nil
}

func (c *Client) dispatch(f frame) {
	payload := structers.Payload{
		ID:        f.ID,
		Op:        f.Op,
		T:         f.T,
		EventType: constant.EventType(f.T),
	}
	if len(f.D) > 0 {
		if err := json.Unmarshal(f.D, &payload.Data); err != nil {
			logger.Errorf("解析事件数据失败 %s: %v", f.T, err)
			return
		}
	}
	middleware.ProcessPayload(payload, c.api)
}

// dropSession 清空本地会话 token，下次握手强制走 IDENTIFY 重新鉴权。
func (c *Client) dropSession() {
	c.mu.Lock()
	c.sessionID = ""
	c.seq = 0
	c.mu.Unlock()
}

// waitForSlot 会话失效后按 /gateway/bot 的 session_start_limit 决定重连时机：
// remaining 还有配额则按短退避重排；配额耗尽则等到 reset_after 再试，避免触发 QQ 会话惩罚。
func (c *Client) waitForSlot() bool {
	info, err := c.api.GatewayBot()
	if err != nil {
		// 限额接口不可用时不阻塞，退化为温和退避继续重试。
		logger.Warnf("查询会话启动限额失败，按默认退避重连: %v", err)
		return c.breathe(jitter(5 * time.Second))
	}
	if info.Limit.Remaining > 0 || info.Limit.ResetAfter <= 0 {
		// 有剩余配额，稍等即重试；表现比为追上 reset_after 防御性多等。
		logger.Warnf("会话失效，剩余启动配额 %d，稍后重新鉴权", info.Limit.Remaining)
		return c.breathe(jitter(3 * time.Second))
	}
	// 配额耗尽：等待配额窗口重置。reset_after 单位为毫秒。
	wait := time.Duration(info.Limit.ResetAfter) * time.Millisecond
	logger.Warnf("会话启动配额耗尽（%d/%d），%.0fm 后重新鉴权",
		info.Limit.Remaining, info.Limit.Total, wait.Minutes())
	return c.breathe(wait)
}

func (c *Client) setConn(conn *websocket.Conn) {
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
}

func (c *Client) closeConn() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func (c *Client) getSession() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

func (c *Client) setSession(id string) {
	c.mu.Lock()
	c.sessionID = id
	c.mu.Unlock()
}

func (c *Client) getSeq() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq
}

// bytesEq 判断 d 帧是否等价于给定布尔值（raw 可能是 true/false 或 "true"/"false"）。
func bytesEq(raw json.RawMessage, want bool) bool {
	switch string(raw) {
	case "true":
		return want
	case "false":
		return !want
	}
	return false
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
