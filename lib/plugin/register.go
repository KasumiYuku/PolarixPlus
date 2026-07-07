package plugin

import (
	"Plrx/lib/context"
	"Plrx/lib/parser"
	"strings"
	"sync"
)

var GlobalCommands map[string]*Command = make(map[string]*Command)
var lock sync.RWMutex = sync.RWMutex{}
var commandCount uint = 0

// 处理所有子指令的回调函数
func subCommandHandle(command *Command, pluginId string) {
	lock.Lock()
	defer lock.Unlock()
	// 处理解析器接口
	if command.Parser == nil {
		command.Parser = &parser.DefaultParser{}
	}
	// 处理PluginId
	command.PluginId = pluginId
	// 递增指令计数
	commandCount++
	if len(command.SubCommand) > 0 {
		// 处理回调函数
		command.SubCommandFallback = command.Handle
		command.Handle = subCommandHandleFunc
		for k := range command.SubCommand {
			subCommandHandle(command.SubCommand[k], pluginId)
		}

	}
}

func Register(plugin *Plugin) {
	lock.Lock()
	defer lock.Unlock()
	for k := range plugin.Commands {
		v := plugin.Commands[k] // 读取指针
		v.PluginId = plugin.Id
		if v.Parser == nil {
			v.Parser = &parser.DefaultParser{} // 如果没有自定义解析器, 使用默认的解析器
		}
		if v.Handle == nil {
			v.Handle = defaultCommandHandle
		}
		if len(v.SubCommand) > 0 {
			// 存在子指令, 替换处理函数
			subCommandHandle(v, plugin.Id)
		} else {
			commandCount++
		}
		GlobalCommands[v.Prefix] = v
	}
}

// 根据前缀获取Command指针
func GetCommand(prefix string) (*Command, bool) {
	lock.RLock()
	defer lock.RUnlock()
	cmd, ok := GlobalCommands[prefix]
	return cmd, ok
}

// 处理包含子指令的指令
func subCommandHandleFunc(context *context.MessageContext) error {
	args := strings.Split(context.Raw, " ") // 一定会有0号元素, 这里已经是传入的指令处理部分了
	currentCmd, ok := GetCommand(args[0])   // 获取父级指令对象
	if !ok || currentCmd == nil {
		return nil
	}
	subCommandPrefixIndex := 1 // 子指令前缀的索引位置
	for {
		if len(currentCmd.SubCommand) == 0 {
			// 叶子指令
			return currentCmd.Handle(context)
		}

		// 无法提取子指令
		if len(args) <= subCommandPrefixIndex {
			if currentCmd.SubCommandFallback == nil {
				return nil
			}
			return currentCmd.SubCommandFallback(context)
		}

		prefix := args[subCommandPrefixIndex] // 子指令前缀
		var targetCommand *Command
		for k := range currentCmd.SubCommand {
			v := currentCmd.SubCommand[k]
			if v.Prefix == prefix {
				// 匹配到子指令
				targetCommand = v
				break
			}
		}

		if targetCommand == nil {
			// 没有找到
			if currentCmd.SubCommandFallback == nil {
				return nil
			}
			return currentCmd.SubCommandFallback(context)
		}

		// 下一个匹配
		currentCmd = targetCommand
		subCommandPrefixIndex++
	}
}

// 获取总指令数
func GetCommandCount() uint {
	return commandCount
}

// 兜底处理函数
func defaultCommandHandle(_ *context.MessageContext) error {
	return nil
}
