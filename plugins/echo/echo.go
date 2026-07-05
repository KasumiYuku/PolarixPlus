package echo

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
)

func init() {

	var commands []*plugin.Command = make([]*plugin.Command, 0)

	commands = append(commands, &plugin.Command{
		Prefix:   "/echo",
		Role:     constant.RoleMember,
		Describe: "回声洞",
		Handle:   echoHandlefunc,
	})

	plugin.Register(&plugin.Plugin{
		Id:       "echo",
		Commands: commands,
	})
}

func echoHandlefunc(context *context.MessageContext) error {
	return context.Markdown(context.Raw).Send()
}
