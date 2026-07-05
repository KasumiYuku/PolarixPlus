package context

import "Plrx/lib/qqapi"

type Context struct {
	*MessageManager
}

// 初始化Context对象及MessageManager对象
func (context *Context) Init(messageId, eventId string, qqapi *qqapi.Client) {
	context.MessageManager = &MessageManager{
		MessageId: messageId,
		EventId:   eventId,
		Qapi:      qqapi,
	}
}

func (context *Context) SetGroupId(id string) {
	context.MessageManager.GroupId = id
}

func (context *Context) SetUserId(id string) {
	context.MessageManager.UserId = id
}
