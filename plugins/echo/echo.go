package echo

import (
	"Plrx/lib/buttons"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/templates"
	"fmt"
)

func init() {

	var commands []*plugin.Command = make([]*plugin.Command, 0)

	commands = append(commands, &plugin.Command{
		Prefix:   "/echo",
		Role:     constant.RoleMember,
		Describe: "回声洞",
		Handle:   echoHandlefunc,
	})

	commands = append(commands, &plugin.Command{
		Prefix:   "/random",
		Role:     constant.RoleMember,
		Describe: "随机图",
		Handle:   randomImg,
	})

	commands = append(commands, &plugin.Command{
		Prefix:   "/uid",
		Role:     constant.RoleMember,
		Describe: "获取UID",
		Handle:   getUid,
	})

	commands = append(commands, &plugin.Command{
		Prefix:         "/gid",
		Role:           constant.RoleMember,
		Describe:       "获取群ID",
		Handle:         getGid,
		DisablePrivate: true,
	})

	commands = append(commands, &plugin.Command{
		Prefix:   "/showcase",
		Role:     constant.RoleMember,
		Describe: "消息构造器演示",
		Handle:   showcase,
	})

	plugin.Register(&plugin.Plugin{
		Id:       "echo",
		Commands: commands,
	})

	buttons.RegisterCallbackFunc("callbacktest", echoButtonCallback)
}

func echoHandlefunc(context *context.MessageContext) error {
	msg := context.Markdown(context.Raw)
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("callbacktest", "回调按钮测试", "点击了", buttons.Gray, 0)
	btn.SetCallbackWithoutHandle(context.Content).SetUnsupportedTip("1").SetUserWhiteList(append(make([]string, 0), context.UserId))
	msg.Keyboard(k)
	return msg.Send()
}

func echoButtonCallback(context *context.CallbackContext) error {
	return context.Markdown(fmt.Sprintf("## 收到回调\n额外数据:\n```\n%v\n```", context.Data)).Send()
}

func randomImg(context *context.MessageContext) error {
	type result struct {
		Url    string `json:"url"`
		Width  uint   `json:"width"`
		Height uint   `json:"height"`
	}
	var re result
	err := context.Request.Get("https://www.loliapi.com/bg/?type=json", &re, nil)
	if err != nil {
		return err
	}
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("1", "再来一张", "还要啊", buttons.Blue, 0)
	btn.SetAutoCommand("/random", true, false).SetUnsupportedTip("不支持按钮捏").SetPermission(buttons.AllUser)
	return context.Msg().
		Image(re.Url, "img", int(re.Width), int(re.Height)).
		Markdown(fmt.Sprintf("> 图片源: [loliapi](https://www.loliapi.com/)\n\n> 图片直链:\n```\n%v\n```", re.Url)).
		Keyboard(k).
		Send()
}

func getUid(context *context.MessageContext) error {
	md, err := context.MarkdownTemplate("UserIdCard", &templates.Args{
		"id":     context.UserId,
		"msg_id": context.MessageId,
	})
	if err != nil {
		return err
	}
	return md.Send()
}

func getGid(context *context.MessageContext) error {
	md := context.Markdown(fmt.Sprintf("## 当前群ID\n```\n%v\n```", context.GroupId))
	return md.Send()
}

// showcase 演示消息
func showcase(ctx *context.MessageContext) error {
	var re struct {
		Url    string `json:"url"`
		Width  uint   `json:"width"`
		Height uint   `json:"height"`
	}
	if err := ctx.Request.Get("https://www.loliapi.com/bg/?type=json", &re, nil); err != nil {
		return err
	}
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("1", "再来一张", "还要啊", buttons.Blue, 0)
	btn.SetAutoCommand("/random", true, false).SetUnsupportedTip("不支持按钮捏").SetPermission(buttons.AllUser)
	return ctx.Msg().
		At(ctx.UserId).
		Text(" 看这个").
		Image(re.Url, "随机图").
		Markdown(fmt.Sprintf("> 图片源: [loliapi](https://www.loliapi.com/)\n\n```\n%v\n```", re.Url)).
		Keyboard(k).
		Send()
}
