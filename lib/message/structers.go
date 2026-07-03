package message

import (
	"Plrx/lib/constant"
	"Plrx/lib/contract"
	"Plrx/lib/qqapi"
	"Plrx/lib/structers"
	"Plrx/lib/templates"
	"encoding/json"
	"fmt"
)

type Message struct {
	structers.BaseMessage
	MsgSeq      uint8                  `json:"msg_swq,omitempty"`
	MsgId       string                 `json:"msg_id,omitempty"`
	EventId     string                 `json:"event_id,omitempty"`
	Type        uint8                  `json:"msg_type"`
	Markdown    templates.Markdown     `json:"markdown,omitzero"`
	KeyboardRaw json.RawMessage        `json:"keyboard,omitempty"`
	keyboard    contract.CanMarshal    `json:"-"`
	qapi        *qqapi.Client          `json:"-"`
	GroupId     string                 `json:"-"`
	UserId      string                 `json:"-"`
	Target      constant.MessageOrigin `json:"-"`
}

type UserMessage struct {
	structers.BaseMessage
}

func (msg *Message) Send() error {
	if msg.MsgId != "" {
		// 是否已经达到了被动回复的上限
		if msg.MsgSeq == 5 {
			return fmt.Errorf("Send 5 message, more message need initiative push")
		}
		// 递增消息ID
		msg.MsgSeq++
	}
	// 匹配消息类型
	switch msg.Target {
	case constant.GroupMessage:
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("Failed when marshal data: %w", err)
		}
		return msg.qapi.SendGroupMessage(data, msg.GroupId)
	case constant.PrivateMessage:
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("Failed when marshal data: %w", err)
		}
		msg.qapi.SendPrivateMessage(data, msg.UserId)
	}
	return nil
}
