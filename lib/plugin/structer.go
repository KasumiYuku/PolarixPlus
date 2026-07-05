package plugin

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/parser"
	"reflect"
)

type CommandHandleFunc func(*context.MessageContext) error

type Command struct {
	Prefix             string                // 指令前缀
	Role               constant.RoleRequired // 最低权限
	Describe           string                // 指令描述
	Handle             CommandHandleFunc     // 处理函数
	PluginId           string                // 属于的插件ID
	SubCommand         []*Command            // 子指令
	SubCommandFallback CommandHandleFunc     // 子指令未找到时回退的函数
	// 解析器
	Parser       parser.Parser // 解析器接口
	ParserTarget reflect.Type  // 解析模板
}

type Plugin struct {
	Id       string
	Commands []*Command
}
