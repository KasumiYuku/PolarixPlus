package message

import (
	"Plrx/lib/contract"
	"Plrx/lib/templates"
	"encoding/json"
)

type MarkdownMessage struct {
	*Message
	Markdown    templates.Markdown  `json:"markdown"`
	KeyboardRaw json.RawMessage     `json:"keyboard,omitempty"`
	keyboard    contract.CanMarshal `json:"-"`
}

// 实现CanMarshal接口
func (msg *MarkdownMessage) Marshal() ([]byte, error) {
	if msg.keyboard != nil {
		keyboardData, err := msg.keyboard.Marshal()
		if err != nil {
			return nil, &JSONMarshalError{Err: err}
		}
		msg.KeyboardRaw = keyboardData
	}
	return json.Marshal(msg)
}

// 设置内容
func (msg *MarkdownMessage) Content(content string) {
	msg.Markdown = templates.Markdown{
		Content: content,
	}
}

// 设置按钮板
func (msg *MarkdownMessage) Keyboard(keyboard contract.CanMarshal) {
	msg.keyboard = keyboard
}

// 初始化Message结构体
func (msg *MarkdownMessage) Init() {
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
