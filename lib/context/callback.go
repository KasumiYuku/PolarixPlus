package context

import "Plrx/lib/qqapi"

// 按钮回调上下文
type CallbackContext struct {
	*Context
	Data     string
	PluginId string
	ButtonId string
}

func (ctx *CallbackContext) Init(eventId string, client *qqapi.Client) {
	ctx.Context = &Context{}
	ctx.Context.Init("", eventId, client)
}

func (ctx *CallbackContext) Done() error {
	return ctx.Context.Qapi.InteracteCallback(ctx.EventId)
}
