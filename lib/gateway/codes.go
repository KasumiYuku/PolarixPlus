package gateway

// 网关 WebSocket 关闭码处置。
const (
	codeAuthFail   = 4004 // 鉴权失败
	codeSessGone   = 4006 // 会话不再有效
	codeInvalidSeq = 4007 // 序号无效
	codeBotOffline = 4914 // 机器人下线
	codeBotBanned  = 4915 // 机器人封禁
)

// canResume 该关闭码下是否允许携带旧会话重连。
func canResume(code int) bool {
	switch code {
	case codeBotOffline, codeBotBanned:
		return false
	}
	return true
}

// needReidentify 该关闭码下会话不可复用, 必须重新鉴权。
func needReidentify(code int) bool {
	switch code {
	case codeAuthFail, codeSessGone, codeInvalidSeq:
		return true
	}
	return false
}
