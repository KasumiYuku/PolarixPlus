package context

import (
	"Plrx/lib/constant"
	"Plrx/lib/message"
	"Plrx/lib/qqapi"
	"Plrx/lib/templates"
)

type MessageManager struct {
	MessageId string
	EventId   string
	GroupId   string
	UserId    string
	Target    constant.MessageOrigin
	Qapi      *qqapi.Client
	ref       *message.MsgRef
}

// 生成包含元信息的消息结构
func (manager *MessageManager) baseStruct() *message.Message {
	msg := &message.Message{
		EventId: manager.EventId,
		MsgId:   manager.MessageId,
		Qapi:    manager.Qapi,
		GroupId: manager.GroupId,
		UserId:  manager.UserId,
		Target:  manager.Target,
	}
	// 确保引用的是同一个计数器
	if manager.ref == nil {
		msg.InitRef()
		manager.ref = msg.MsgRef
	} else {
		msg.MsgRef = manager.ref
	}
	return msg
}

// 生成纯文本回复消息
func (manager *MessageManager) Text(content string) *message.TextMessage {
	metamsg := manager.baseStruct()
	msg := message.TextMessage{
		Message: metamsg,
	}
	// 填充内容
	msg.Type = constant.PlainText
	msg.Content(content)
	msg.Init()
	return &msg
}

// 生成Markdown回复消息
func (manager *MessageManager) Markdown(content string) *message.MarkdownMessage {
	metamsg := manager.baseStruct()
	msg := message.MarkdownMessage{
		Message: metamsg,
	}
	msg.Init()
	// 填充内容
	msg.Type = constant.Markdown
	msg.Markdown = templates.Markdown{
		Content: content,
	}
	return &msg
}

// 填充Markdown模板并构造Markdown消息
func (manager *MessageManager) MarkdownTemplate(id string, args *templates.Args) (*message.MarkdownMessage, error) {
	// 填充Markdown模板
	content, err := templates.FillMarkdownTemplate(id, *args)
	if err != nil {
		return nil, err
	}
	// 构造消息
	return manager.Markdown(content), nil
}

// 填充Markdown模板时出错会panic
func (manager *MessageManager) UnsafeMarkdownTemplate(id string, args *templates.Args) *message.MarkdownMessage {
	content, err := templates.FillMarkdownTemplate(id, *args)
	if err != nil {
		panic(err)
	}
	return manager.Markdown(content)
}
