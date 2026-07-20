package middleware

import (
	"Plrx/lib/buttons"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/message"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/structers"
	"Plrx/lib/utils"
	"log"
	"reflect"
	"strings"
)

func ProcessPayload(payload structers.Payload, client *qqapi.Client) {
	// js, _ := json.MarshalIndent(payload, "", "")
	// log.Printf("[Debug]Raw request: %v", string(js))
	switch payload.EventType {
	case constant.GROUP_AT_MESSAGE_CREATE, constant.GROUP_MESSAGE_CREATE:
		// payload.Data.Content = strings.TrimSpace(payload.Data.Content)
		raw := payload.Data.Content
		payload.Data.Content = utils.FilterAt(payload.Data.Content)
		msgs := strings.Split(payload.Data.Content, " ")
		var prefix = msgs[0]
		if len(msgs) > 1 && (strings.HasPrefix(msgs[0], "\u003c@") || strings.HasPrefix(msgs[0], "<@")) && (strings.HasSuffix(msgs[0], ">")) {
			prefix = msgs[1]
		}
		cmd, ok := plugin.GetCommand(prefix)
		if ok {
			// log.Printf("捕获到%v指令, 来自插件: %v", cmd.Prefix, cmd.PluginId)
			if !cmd.Role.CanUse(payload.Data.Author.Role) {
				log.Printf("用户%v无权限使用%v指令", payload.Data.Author.Username, cmd.Prefix)
				return
			}

			// 解析器
			var parsed any
			if cmd.ParserTarget != nil {
				result := reflect.New(cmd.ParserTarget)
				err := cmd.Parser.Parse(payload.Data.Content, result.Interface())
				if err != nil {
					return
				}
				parsed = result.Interface()
			} else {
				var result string
				err := cmd.Parser.Parse(payload.Data.Content, &result)
				if err != nil {
					return
				}
				parsed = result
			}
			// 构建上下文对象
			ctx := context.MessageContext{
				Parsed: parsed,
				UserMessage: message.UserMessage{
					Content: payload.Data.Content,
				},
				Raw: raw,
			}
			// 初始化消息管理器
			metaContext := &context.Context{}
			metaContext.Init(payload.Data.Id, payload.ID, client)
			ctx.Context = metaContext
			ctx.Init(payload.Data.Id, payload.ID, client)
			ctx.BindStorage(cmd.PluginId, cmd.Prefix)
			ctx.SetGroupId(payload.Data.GroupOpenID)
			ctx.SetUserId(payload.Data.Author.UnionID)
			// err := cmd.Handle(&ctx)
			go messageRecoveryFunc(cmd, &ctx)
		}
	case constant.INTERACTION_CREATE:
		data := payload.Data.Callback.Resolved.ButtonData
		buttonId := payload.Data.Callback.Resolved.ButtonId
		// log.Printf("收到回调按钮推送, 按钮ID = %v", buttonId)
		callbackFunc, ok := buttons.GetCallbackFunc(buttonId)
		if !ok {
			log.Printf("回调按钮: %v未注册回调函数, 跳过处理", buttonId)
			return
		}
		// 找到了回调函数
		ctx := &context.CallbackContext{}
		ctx.Init(payload.ID, client)
		ctx.ButtonId = buttonId
		ctx.Data = data
		ctx.GroupId = payload.Data.GroupOpenID
		go callbackHandleFunc(callbackFunc, ctx)
	}
}

func messageRecoveryFunc(cmd *plugin.Command, context *context.MessageContext) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("在执行指令%v (插件: %v)时出现panic: %v", cmd.Prefix, cmd.PluginId, r)
		}
	}()
	if err := cmd.Handle(context); err != nil {
		log.Printf("在执行指令%v (插件: %v)时出现error: %v", cmd.Prefix, cmd.PluginId, err)
	}
}

func callbackHandleFunc(handle buttons.CallbackButtonHandleFunc, ctx *context.CallbackContext) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("在执行回调按钮: %v 处理函数时候出现panic: %v", ctx.ButtonId, r)
		}
	}()
	if err := ctx.Done(); err != nil {
		log.Printf("在处理回调按钮上报: %v 时候出现error: %v", ctx.ButtonId, err)
	}
	if err := handle(ctx); err != nil {
		log.Printf("在执行回调按钮: %v 处理函数时候出现error: %v", ctx.ButtonId, err)
	}
}
