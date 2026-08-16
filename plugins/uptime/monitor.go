package uptime

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/templates"
	"fmt"
)

func init() {

	// schedule.Register(&schedule.Job{
	// 	Id:        "uptime-schedule",
	// 	PluginId:  "uptime",
	// 	Interval:  time.Minute * 3,
	// 	GroupId:   "4B52E5B916572A658E73E0ABA13DF283",
	// 	Immediate: false,
	// 	Handle:    handle,
	// })

	subcommand := make([]*plugin.Command, 0)
	subcommand = append(subcommand, &plugin.Command{
		Prefix: "add",
		Role:   constant.RoleAdmin,
	})

	command := make([]*plugin.Command, 0)
	command = append(command, &plugin.Command{
		Prefix:     "/uptime",
		Role:       constant.RoleAdmin,
		Handle:     helpText,
		SubCommand: subcommand,
	})

	self := &plugin.Plugin{
		Id:       "uptime",
		Commands: command,
	}
	plugin.Register(self)
}

func handle(ctx *context.ScheduleContext) error {

	// 获取所有监测域名

	// fmt.Printf()
	err := ctx.Request.Get("https://api.yearnstudio.cn/", nil, nil)
	if err == nil {
		return ctx.Markdown("## 定时Uptime检测结果\n🟩正常").Send()
	} else {
		return ctx.Markdown(fmt.Sprintf("## 定时Uptime检测结果\n🟥异常\n\n> %v", err)).Send()
	}
}

func helpText(ctx *context.MessageContext) error {
	// fmt.Printf("执行了helpText")
	md, err := ctx.MarkdownTemplate("UptimeHelp", &templates.Args{})
	if err != nil {
		return err
	}
	return md.Send()
}

func addSite(ctx *context.MessageContext) error {
	fmt.Printf("执行了helpText, content = ")
	return nil
}
