package message

import (
	"Plrx/lib/constant"
	"Plrx/lib/contract"
	"Plrx/lib/qqapi"
	"encoding/json"
	"fmt"
	"sync"
)

type Message struct {
	*MsgRef
	MsgId            string                 `json:"msg_id,omitempty"`
	MsgSeq           uint8                  `json:"msg_seq,omitempty"`
	EventId          string                 `json:"event_id,omitempty"`
	Type             constant.MessageType   `json:"msg_type"`
	Qapi             *qqapi.Client          `json:"-"`
	GroupId          string                 `json:"-"`
	UserId           string                 `json:"-"`
	Target           constant.MessageOrigin `json:"-"` // 发送目标(私聊/群)
	used             bool                   `json:"-"` // 是否被使用过
	MarshalInterface contract.CanMarshal    `json:"-"`
	initiativePush   bool                   `json:"-"` // 是否为主动推送
}

// 初始化回复计数器
func (msg *Message) InitRef() *Message {
	msg.MsgRef = &MsgRef{
		msgSeq: 1,
		lock:   sync.Mutex{},
	}
	return msg
}

type UserMessage struct {
	Content string
}

// 发送消息
func (msg *Message) Send() error {
	// 此消息是否已被使用
	if msg.used {
		return &MessageUsed{
			MessageId: msg.MsgId,
		}
	}
	// 尝试增加计数
	seq, err := msg.Count()
	// 失败
	if err != nil {
		return err
	}
	// 设置消息编号
	msg.MsgSeq = seq

	// 解析消息
	var data []byte
	// 如果有传入解析接口
	if msg.MarshalInterface != nil {
		data, err = msg.MarshalInterface.Marshal()
	} else {
		// 否则使用内建
		data, err = json.Marshal(msg)
	}
	// 解析出错
	if err != nil {
		return &JSONMarshalError{
			Err: err,
		}
	}

	if msg.Qapi == nil {
		panic("QQAPI Clinet空指针异常")
	}

	// 匹配消息类型
	switch msg.Target {
	case constant.GroupMessage:
		return msg.Qapi.SendGroupMessage(data, msg.GroupId)
	case constant.PrivateMessage:
		return msg.Qapi.SendPrivateMessage(data, msg.UserId)
	default:
		// TODO: 更换为类型
		return fmt.Errorf("Unknown message target type: %v", msg.Target)
	}
}

// 设置为主动推送消息
func (msg *Message) SetInitiativeMessage() {
	msg.initiativePush = true
}
