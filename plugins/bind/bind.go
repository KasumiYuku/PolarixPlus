package bind

import (
	"Plrx/lib/buttons"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/templates"
	"fmt"
	"strings"
)

func init() {
	commands := make([]*plugin.Command, 0)
	subCommands := make([]*plugin.Command, 0)
	subCommands = append(subCommands, &plugin.Command{
		Prefix: "confirm",
		Handle: confirmCode,
		Role:   constant.RoleMember,
	})
	commands = append(commands, &plugin.Command{
		Prefix:     "/bind",
		Role:       constant.RoleMember,
		SubCommand: subCommands,
		Handle:     startBind,
	})
	self := &plugin.Plugin{}
	self.Commands = commands
	self.Id = "bind"
	plugin.Register(self)
}

func startBind(ctx *context.MessageContext) error {
	args := strings.Split(ctx.Content, " ")
	if len(args) < 2 {
		return ctx.Text("请使用/绑定 [UID]来开始绑定流程").Send()
	}
	msg := ctx.UnsafeMarkdownTemplate("BindGuide", &templates.Args{"uid": args[1]})
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("2", "点击我输入UID", "请输入UID", buttons.Blue, 0)
	btn.SetAutoCommand(fmt.Sprintf("/bind confirm %v ", args[1]), false, false).SetUserWhiteList(append(make([]string, 0), ctx.UserId)).SetUnsupportedTip("不支持按钮")
	msg.Keyboard(k)
	return msg.Send()
}

func confirmCode(ctx *context.MessageContext) error {
	args := strings.Split(ctx.Content, " ")
	if len(args) < 4 {
		return ctx.Text("指令不正确\n请使用/绑定 [UID]来开始绑定流程").Send()
	}
	msg := ctx.UnsafeMarkdownTemplate("BindConfirm", &templates.Args{"uid": args[2], "code": args[3]})
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("confirm", "确认无误", "已确认", buttons.Blue, 0)
	btn.SetCallback(fmt.Sprintf("%v %v", args[2], args[3]), cb).SetUserWhiteList(append(make([]string, 0), ctx.UserId)).SetUnsupportedTip("不支持按钮")
	msg.Keyboard(k)
	return msg.Send()
}

func cb(ctx *context.CallbackContext) error {
	return ctx.Text(fmt.Sprintf("已模拟绑定: %v", ctx.Data)).Send()
}
