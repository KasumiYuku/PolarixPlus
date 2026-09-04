package qqapi

import "encoding/json"

// 错误码中文提示
var errorHints = map[int]string{
	100016: "secret 输入错误",
	10004:  "appid 输入错误",
	100007: "机器人被封禁或不存在",
	100017: "接口调用超过频率限制",
	11300:  "仅私域机器人可用（link type check failed）",
	11700:  "机器人已取消该能力",
	11253:  "按钮回调权限不足",
	630006: "header appid 获取失败",
	304023: "消息正在审核中（audit）",
}

// Describe 返回 QQ 平台错误码的中文描述。
func Describe(code int) string {
	if hint, ok := errorHints[code]; ok {
		return hint
	}
	return "未知错误"
}

// IsRetryable 判断是否应重试的业务错误码。
func IsRetryable(code int, retryWhen []int) bool {
	for _, c := range retryWhen {
		if c == code {
			return true
		}
	}
	return false
}

// QQError 解析 QQ 响应中的错误码。
type QQError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *QQError) Error() string {
	return Describe(e.Code) + " " + e.Message
}

// ParseError 从响应字节中提取 QQ 错误。非错误时返回 nil。
func ParseError(body []byte) *QQError {
	var e QQError
	if err := json.Unmarshal(body, &e); err != nil {
		return nil
	}
	if e.Code == 0 {
		return nil
	}
	return &e
}
