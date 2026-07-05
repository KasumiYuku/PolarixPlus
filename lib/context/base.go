package context

import (
	"Plrx/lib/qqapi"
	"Plrx/lib/requests"
)

type Context struct {
	*MessageManager
	Request *requests.Client
}

// 初始化Context对象及MessageManager对象
func (context *Context) Init(messageId, eventId string, qqapi *qqapi.Client) {
	context.MessageManager = &MessageManager{
		MessageId: messageId,
		EventId:   eventId,
		Qapi:      qqapi,
	}
	context.Request = qqapi.Request
}

func (context *Context) SetGroupId(id string) {
	context.MessageManager.GroupId = id
}

func (context *Context) SetUserId(id string) {
	context.MessageManager.UserId = id
}
