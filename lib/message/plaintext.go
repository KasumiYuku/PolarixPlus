package message

import "encoding/json"

type TextMessage struct {
	*Message
	TextContent string `json:"content"`
}

// 设置内容
func (msg *TextMessage) Content(content string) *TextMessage {
	msg.TextContent = content
	return msg
}

// 实现CanMarshal
func (msg *TextMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

// 初始化Message结构体
func (msg *TextMessage) Init() {
	var metamsg *Message
	// 初始化新的Messgae
	if msg.Message == nil {
		metamsg = &Message{}
		metamsg.InitRef()
	} else {
		// 已有, 重用
		metamsg = msg.Message
	}
	// 建立Marshal接口传递
	metamsg.MarshalInterface = msg
	// 储存Message指针
	msg.Message = metamsg
}

func NewTextMessage() *TextMessage {
	var msg *TextMessage = &TextMessage{}
	msg.Init()
	return msg
}
