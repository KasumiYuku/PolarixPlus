package plugin

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/parser"
	"Plrx/lib/utils"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var GlobalCommands map[string]*Command = make(map[string]*Command)
var globalPlugins = make(map[string]*Plugin)
var pluginSettings = make(map[string]map[string]any)
var pluginAccess = make(map[string]AccessConfig)
var lock sync.RWMutex = sync.RWMutex{}
var commandCount uint = 0

// 处理所有子指令的回调函数
func subCommandHandle(command *Command, pluginId string) {
	// lock.Lock()
	// defer lock.Unlock()
	// 处理解析器接口
	if command.Parser == nil {
		command.Parser = &parser.DefaultParser{}
	}
	// 处理PluginId
	command.PluginId = pluginId
	// 递增指令计数
	commandCount++
	if command.Handle == nil {
		command.Handle = defaultCommandHandle
	}
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
	globalPlugins[plugin.Id] = plugin
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

type ConfiguredPlugin struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Fields      []ConfigField  `json:"fields"`
	Values      map[string]any `json:"values"`
}

type AccessRule struct {
	Mode   string   `json:"mode"`
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

type AccessConfig struct {
	Default  AccessRule            `json:"default"`
	Commands map[string]AccessRule `json:"commands"`
}

type ManagedPlugin struct {
	ConfiguredPlugin
	Commands []string     `json:"commands"`
	Access   AccessConfig `json:"access"`
}

func LoadConfigurations(settings map[string]map[string]any) error {
	lock.Lock()
	defer lock.Unlock()
	for id, registered := range globalPlugins {
		if len(registered.Config) == 0 {
			continue
		}
		values := cloneSettings(settings[id])
		if registered.ValidateConfig != nil {
			if err := registered.ValidateConfig(values); err != nil {
				return fmt.Errorf("validate configuration for plugin %s: %w", id, err)
			}
		}
		if registered.ApplyConfig != nil {
			if err := registered.ApplyConfig(values); err != nil {
				return fmt.Errorf("apply configuration for plugin %s: %w", id, err)
			}
		}
		pluginSettings[id] = values
	}
	return nil
}

func LoadAccessConfigurations(configs map[string]AccessConfig) error {
	lock.Lock()
	defer lock.Unlock()
	for id, registered := range globalPlugins {
		access := normalizeAccessConfig(configs[id])
		if err := validateAccessRule(access.Default); err != nil {
			return fmt.Errorf("validate access for plugin %s default rule: %w", id, err)
		}
		validCommands := commandPathSet(registered)
		for path, rule := range access.Commands {
			if !validCommands[path] {
				return fmt.Errorf("validate access for plugin %s: unknown command path %s", id, path)
			}
			if err := validateAccessRule(rule); err != nil {
				return fmt.Errorf("validate access for plugin %s command %s: %w", id, path, err)
			}
			if rule.Mode == "off" {
				delete(access.Commands, path)
			}
		}
		pluginAccess[id] = access
	}
	return nil
}

func ConfiguredPlugins() []ConfiguredPlugin {
	lock.RLock()
	defer lock.RUnlock()
	result := make([]ConfiguredPlugin, 0)
	for id, registered := range globalPlugins {
		if len(registered.Config) == 0 {
			continue
		}
		values := cloneSettings(pluginSettings[id])
		for _, field := range registered.Config {
			if field.Type == "password" {
				_, configured := values[field.Key].(string)
				values[field.Key] = configured && values[field.Key] != ""
			}
		}
		name := registered.Name
		if name == "" {
			name = id
		}
		result = append(result, ConfiguredPlugin{ID: id, Name: name, Description: registered.Description, Fields: registered.Config, Values: values})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func ManagedPlugins() []ManagedPlugin {
	configured := ConfiguredPlugins()
	byID := make(map[string]ConfiguredPlugin, len(configured))
	for _, item := range configured {
		byID[item.ID] = item
	}

	lock.RLock()
	defer lock.RUnlock()
	result := make([]ManagedPlugin, 0, len(globalPlugins))
	for id, registered := range globalPlugins {
		base, ok := byID[id]
		if !ok {
			name := registered.Name
			if name == "" {
				name = id
			}
			base = ConfiguredPlugin{ID: id, Name: name, Description: registered.Description, Fields: []ConfigField{}, Values: map[string]any{}}
		}
		commands := make([]string, 0)
		for _, command := range registered.Commands {
			collectCommandPaths(command, command.Prefix, &commands)
		}
		sort.Strings(commands)
		result = append(result, ManagedPlugin{ConfiguredPlugin: base, Commands: commands, Access: cloneAccessConfig(pluginAccess[id])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func ManagedPluginByID(id string) (ManagedPlugin, bool) {
	for _, managed := range ManagedPlugins() {
		if managed.ID == id {
			return managed, true
		}
	}
	return ManagedPlugin{}, false
}

func PrepareAccessConfiguration(id string, access AccessConfig) (AccessConfig, error) {
	lock.RLock()
	registered, ok := globalPlugins[id]
	lock.RUnlock()
	if !ok {
		return AccessConfig{}, fmt.Errorf("plugin %s is not registered", id)
	}

	validCommands := commandPathSet(registered)
	prepared := normalizeAccessConfig(access)
	if err := validateAccessRule(prepared.Default); err != nil {
		return AccessConfig{}, fmt.Errorf("default rule: %w", err)
	}
	for path, rule := range prepared.Commands {
		if !validCommands[path] {
			return AccessConfig{}, fmt.Errorf("unknown command path: %s", path)
		}
		if err := validateAccessRule(rule); err != nil {
			return AccessConfig{}, fmt.Errorf("command %s: %w", path, err)
		}
		if rule.Mode == "off" {
			delete(prepared.Commands, path)
		}
	}
	return prepared, nil
}

func ApplyAccessConfiguration(id string, access AccessConfig) {
	lock.Lock()
	defer lock.Unlock()
	pluginAccess[id] = cloneAccessConfig(access)
}

func CanUse(pluginID, commandPath, userID, groupID string) bool {
	lock.RLock()
	defer lock.RUnlock()
	access := pluginAccess[pluginID]
	rule, overridden := access.Commands[commandPath]
	if !overridden {
		rule = access.Default
	}
	userMatched := contains(rule.Users, userID)
	groupMatched := contains(rule.Groups, groupID)
	switch rule.Mode {
	case "whitelist":
		return userMatched || groupMatched
	case "blacklist":
		return !userMatched && !groupMatched
	default:
		return true
	}
}

func ResolveCommandPath(command *Command, raw string) string {
	_, path := ResolveCommand(command, raw)
	return path
}

func ResolveCommand(command *Command, raw string) (*Command, string) {
	path := command.Prefix
	current := command
	parts := strings.Fields(utils.FilterAt(raw))
	for index := 1; index < len(parts) && len(current.SubCommand) > 0; index++ {
		var next *Command
		for _, candidate := range current.SubCommand {
			if candidate.Prefix == parts[index] {
				next = candidate
				break
			}
		}
		if next == nil {
			break
		}
		path += " " + next.Prefix
		current = next
	}
	return current, path
}

func collectCommandPaths(command *Command, path string, result *[]string) {
	*result = append(*result, path)
	for _, subcommand := range command.SubCommand {
		collectCommandPaths(subcommand, path+" "+subcommand.Prefix, result)
	}
}

func commandPathSet(registered *Plugin) map[string]bool {
	result := make(map[string]bool)
	for _, command := range registered.Commands {
		paths := make([]string, 0)
		collectCommandPaths(command, command.Prefix, &paths)
		for _, path := range paths {
			result[path] = true
		}
	}
	return result
}

func normalizeAccessConfig(access AccessConfig) AccessConfig {
	access.Default = normalizeAccessRule(access.Default)
	commands := make(map[string]AccessRule, len(access.Commands))
	for path, rule := range access.Commands {
		commands[strings.TrimSpace(path)] = normalizeAccessRule(rule)
	}
	access.Commands = commands
	return access
}

func normalizeAccessRule(rule AccessRule) AccessRule {
	rule.Mode = strings.ToLower(strings.TrimSpace(rule.Mode))
	if rule.Mode == "" {
		rule.Mode = "off"
	}
	rule.Users = cleanIDs(rule.Users)
	rule.Groups = cleanIDs(rule.Groups)
	return rule
}

func cleanIDs(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func validateAccessRule(rule AccessRule) error {
	if rule.Mode != "off" && rule.Mode != "whitelist" && rule.Mode != "blacklist" {
		return fmt.Errorf("mode must be off, whitelist, or blacklist")
	}
	return nil
}

func contains(values []string, target string) bool {
	if target == "" {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneAccessConfig(source AccessConfig) AccessConfig {
	result := AccessConfig{Default: cloneAccessRule(source.Default), Commands: make(map[string]AccessRule, len(source.Commands))}
	for path, rule := range source.Commands {
		result.Commands[path] = cloneAccessRule(rule)
	}
	return result
}

func cloneAccessRule(source AccessRule) AccessRule {
	return AccessRule{Mode: source.Mode, Users: append([]string(nil), source.Users...), Groups: append([]string(nil), source.Groups...)}
}

func PrepareConfiguration(id string, input map[string]any) (map[string]any, error) {
	lock.RLock()
	defer lock.RUnlock()
	registered, ok := globalPlugins[id]
	if !ok || len(registered.Config) == 0 {
		return nil, fmt.Errorf("plugin %s has no configurable options", id)
	}
	current := pluginSettings[id]
	prepared := make(map[string]any, len(registered.Config))
	for _, field := range registered.Config {
		value, exists := input[field.Key]
		if field.Type == "password" && (!exists || value == "") {
			value, exists = current[field.Key]
		}
		if !exists {
			if field.Type == "boolean" {
				value = false
			} else {
				value = ""
			}
		}
		if field.Type == "boolean" {
			if _, ok := value.(bool); !ok {
				return nil, fmt.Errorf("field %s must be a boolean", field.Key)
			}
		} else if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("field %s must be a string", field.Key)
		}
		prepared[field.Key] = value
	}
	if registered.ValidateConfig != nil {
		if err := registered.ValidateConfig(cloneSettings(prepared)); err != nil {
			return nil, err
		}
	}
	return prepared, nil
}

func ApplyConfiguration(id string, settings map[string]any) error {
	lock.Lock()
	defer lock.Unlock()
	registered, ok := globalPlugins[id]
	if !ok {
		return fmt.Errorf("plugin %s is not registered", id)
	}
	if registered.ApplyConfig != nil {
		if err := registered.ApplyConfig(cloneSettings(settings)); err != nil {
			return err
		}
	}
	pluginSettings[id] = cloneSettings(settings)
	return nil
}

func cloneSettings(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
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
	// args := strings.Split(context.Raw, " ")
	args := strings.Split(utils.FilterAt(context.Raw), " ") // 一定会有0号元素, 这里已经是传入的指令处理部分了
	currentCmd, ok := GetCommand(args[0])                   // 获取父级指令对象
	if !ok || currentCmd == nil {
		return nil
	}
	commandPath := currentCmd.Prefix
	context.BindStorage(currentCmd.PluginId, commandPath)
	subCommandPrefixIndex := 1 // 子指令前缀的索引位置
	for {
		if currentCmd.Handle == nil || len(currentCmd.SubCommand) == 0 {
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
		if context.MessageManager.Target == constant.PrivateMessage && targetCommand.DisablePrivate {
			return nil
		}

		// 下一个匹配
		currentCmd = targetCommand
		commandPath += " " + currentCmd.Prefix
		context.BindStorage(currentCmd.PluginId, commandPath)
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
