package echo

import (
	buttons "Plrx/lib/button"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
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

	plugin.Register(&plugin.Plugin{
		Id:       "echo",
		Commands: commands,
	})
}

func echoHandlefunc(context *context.MessageContext) error {
	return context.Markdown(context.Raw).Send()
}

func randomImg(context *context.MessageContext) error {
	type result struct {
		Url    string `json:"url"`
		Witdh  uint   `json:"width"`
		Height uint   `json:"height"`
	}
	var re result
	err := context.Request.Get("https://www.loliapi.com/bg/?type=json", &re, nil)
	if err != nil {
		return err
	}
	msg := context.Markdown(fmt.Sprintf("![img #%v #%v](%v)\n> 图片源: [loliapi](https://www.loliapi.com/)\n> 图片直链:\n```\n%v\n```", re.Witdh, re.Height, re.Url, re.Url))
	k := &buttons.Keyboard{}
	btn, _ := k.AppendButton("1", "再来一张", "还要啊", buttons.Blue, 0)
	btn.SetAutoCommand("/random", true, false).SetUnsupportedTip("不支持按钮捏").SetPermission(buttons.AllUser)
	msg.Keyboard(k)
	msg.Send()
	// context.Text(fmt.Sprintf("![img #%v #%v](%v)\n> 图片源: [loliapi](https://www.loliapi.com/)\n> Origin:\n```\n%v\n```", re.Witdh, re.Height, re.Url, re.Url)).Send()
	return nil
}
