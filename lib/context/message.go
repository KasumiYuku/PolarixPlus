package context

import (
	"Plrx/lib/message"
	"Plrx/lib/qqapi"
)

type MessageContext struct {
	*Context
	message.UserMessage
	Raw    string // 原始消息
	Parsed any    // 解析后
}

func (ctx *MessageContext) Init(messageId, eventId string, client *qqapi.Client) {
	ctx.Context = &Context{}
	ctx.Context.Init(messageId, eventId, client)
}
