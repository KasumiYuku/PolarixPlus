package context

import (
	"Plrx/lib/constant"
	"Plrx/lib/qqapi"
	"Plrx/lib/requests"
	"Plrx/lib/storage"
)

type Context struct {
	*MessageManager
	Request        *requests.Client
	GlobalStorage  *storage.Store
	PluginStorage  *storage.Store
	CommandStorage *storage.Store
}

// 初始化Context对象及MessageManager对象
func (context *Context) Init(messageId, eventId string, qqapi *qqapi.Client) {
	context.MessageManager = &MessageManager{
		MessageId: messageId,
		EventId:   eventId,
		Qapi:      qqapi,
	}
	context.Request = qqapi.Request
	context.GlobalStorage = storage.Global()
}

// BindStorage exposes namespaces bound to the command currently being handled.
func (context *Context) BindStorage(pluginID, commandID string) {
	context.PluginStorage = storage.Plugin(pluginID)
	context.CommandStorage = storage.Command(pluginID, commandID)
}

func (context *Context) SetGroupId(id string) {
	context.MessageManager.GroupId = id
}

func (context *Context) SetUserId(id string) {
	context.MessageManager.UserId = id
}

func (context *Context) SetMessageOrigin(origin constant.MessageOrigin) {
	context.MessageManager.Target = origin
}
