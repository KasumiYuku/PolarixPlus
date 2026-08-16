package structers

import (
	"Plrx/lib/constant"
	"Plrx/lib/message"
)

// 推送内容解析
type Payload struct {
	ID        string             `json:"id"`
	Op        int                `json:"op"`
	Data      PrasedData         `json:"d"`
	T         string             `json:"t"`
	EventType constant.EventType `json:"-"`
}

type CallbackData struct {
	ButtonData string `json:"button_data"`
	ButtonId   string `json:"button_id"`
}

type PrasedData struct {
	Id          string               `json:"id"`
	Content     string               `json:"content"`
	GroupOpenID string               `json:"group_openid"`
	Attachments []message.Attachment `json:"attachments"`
	Author      struct {
		ID           string                `json:"id"`
		UserOpenID   string                `json:"user_openid"`
		MemberOpenID string                `json:"member_openid"`
		UnionID      string                `json:"union_openid"`
		Role         constant.RoleRequired `json:"member_role"`
		Username     string                `json:"username"`
		Bot          bool                  `json:"bot"`
	} `json:"author"`
	// 用于 Op=13 时的网络探测数据结构
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
	// 回调按钮
	Scene    string `json:"scene"`
	Callback struct {
		Resolved CallbackData `json:"resolved"`
	} `json:"data"`
}
