package structers

import "Plrx/lib/constant"

// 推送内容解析
type Payload struct {
	ID        string             `json:"id"`
	Op        int                `json:"op"`
	Data      PrasedData         `json:"d"`
	T         string             `json:"t"`
	EventType constant.EventType `json:"-"`
}

type PrasedData struct {
	Id          string `json:"id"`
	Content     string `json:"content"`
	GroupOpenID string `json:"group_openid"`
	Author      struct {
		UnionID  string                `json:"union_openid"`
		Role     constant.RoleRequired `json:"member_role"`
		Username string                `json:"username"`
		MemberId string
	} `json:"author"`
	// 用于 Op=13 时的网络探测数据结构
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
}
