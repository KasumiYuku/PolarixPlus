package context

import (
	"Plrx/lib/qqapi"
	"time"
)

// ScheduleContext 定时任务回调上下文 (无关联用户消息, 发送需主动推送)
type ScheduleContext struct {
	*Context
	JobId    string
	PluginId string
	FiredAt  time.Time
}

func (ctx *ScheduleContext) Init(client *qqapi.Client) {
	ctx.Context = &Context{}
	ctx.Context.Init("", "", client)
	ctx.FiredAt = time.Now()
}
