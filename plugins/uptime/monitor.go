package uptime

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/templates"
	"fmt"
	"strings"
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

	command = append(command, &plugin.Command{
		Prefix: "/openai",
		Handle: openAIStatus,
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

func openAIStatus(ctx *context.MessageContext) error {
	type Component struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type AffectedComponent struct {
		ComponentID       string `json:"component_id"`
		OperationalStatus string `json:"operational_status"`
	}
	type Incident struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Impact string `json:"impact"`
	}

	var data struct {
		Summary struct {
			Components         []Component         `json:"components"`
			AffectedComponents []AffectedComponent `json:"affected_components"`
			OngoingIncidents   []Incident          `json:"ongoing_incidents"`
		} `json:"summary"`
	}

	err := ctx.Request.Get("https://status.openai.com/proxy/status.openai.com", &data, nil)
	if err != nil {
		return err
	}

	summary := data.Summary
	// 没有任何故障事件且没有受影响组件
	if len(summary.AffectedComponents) == 0 && len(summary.OngoingIncidents) == 0 {
		return ctx.Text("✅ OpenAI 当前所有服务均运行正常").Send()
	}

	// 建立 ID -> Name 映射表
	nameMap := make(map[string]string, len(summary.Components))
	for _, c := range summary.Components {
		nameMap[c.ID] = c.Name
	}

	// 状态枚举中文映射
	statusText := func(status string) string {
		switch status {
		case "degraded_performance":
			return "性能下降 ⚠️"
		case "partial_outage":
			return "部分中断 ❌"
		case "major_outage":
			return "严重宕机 🚨"
		case "under_maintenance":
			return "正在维护 🛠️"
		default:
			return status
		}
	}

	var sb strings.Builder
	sb.WriteString("⚠️ **OpenAI 服务异常提醒**\n")

	// 1. 正在发生的故障事件
	if len(summary.OngoingIncidents) > 0 {
		sb.WriteString("\n📌 **故障通报：**")
		for _, inc := range summary.OngoingIncidents {
			sb.WriteString(fmt.Sprintf("\n- %s (%s)", inc.Name, inc.Status))
		}
	}

	// 2. 受影响的具体组件
	if len(summary.AffectedComponents) > 0 {
		sb.WriteString("\n\n📉 **受影响服务：**")
		for _, v := range summary.AffectedComponents {
			name := nameMap[v.ComponentID]
			if name == "" {
				name = v.ComponentID
			}
			sb.WriteString(fmt.Sprintf("\n- **%s**: %s", name, statusText(v.OperationalStatus)))
		}
	}

	return ctx.Markdown(sb.String()).Send()
}
