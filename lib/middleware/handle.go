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
	"fmt"
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
			userID := payload.Data.Author.MemberOpenID
			if userID == "" {
				userID = payload.Data.Author.UnionID
			}
			ctx := context.MessageContext{
				UserMessage: message.UserMessage{
					Content:     payload.Data.Content,
					Attachments: payload.Data.Attachments,
				},
				Raw: raw,
			}
			ctx.Init(payload.Data.Id, payload.ID, client)
			ctx.BindStorage(cmd.PluginId, cmd.Prefix)
			ctx.SetGroupId(payload.Data.GroupOpenID)
			ctx.SetUserId(userID)
			ctx.SetMessageOrigin(constant.GroupMessage)
			targetCommand, commandPath := plugin.ResolveCommand(cmd, payload.Data.Content)
			if !cmd.Role.CanUse(payload.Data.Author.Role) {
				log.Printf("用户%v无权限使用%v指令", payload.Data.Author.Username, cmd.Prefix)
				permissionDenied(cmd, &ctx)
				return
			}
			if targetCommand != cmd && !targetCommand.Role.CanUse(payload.Data.Author.Role) {
				log.Printf("用户%v无权限使用%v指令", payload.Data.Author.Username, commandPath)
				permissionDenied(targetCommand, &ctx)
				return
			}
			if !plugin.CanUse(cmd.PluginId, commandPath, userID, payload.Data.GroupOpenID) {
				permissionDenied(targetCommand, &ctx)
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
			ctx.Parsed = parsed
			// err := cmd.Handle(&ctx)
			go messageRecoveryFunc(cmd, targetCommand, &ctx)
		}
	case constant.C2C_MESSAGE_CREATE:
		raw := payload.Data.Content
		payload.Data.Content = strings.TrimSpace(payload.Data.Content)
		if payload.Data.Content == "" {
			return
		}

		prefix := strings.Fields(payload.Data.Content)[0]
		cmd, ok := plugin.GetCommand(prefix)
		if !ok {
			return
		}
		ctx := context.MessageContext{
			UserMessage: message.UserMessage{
				Content:     payload.Data.Content,
				Attachments: payload.Data.Attachments,
			},
			Raw: raw,
		}
		ctx.Init(payload.Data.Id, payload.ID, client)
		ctx.BindStorage(cmd.PluginId, cmd.Prefix)
		ctx.SetUserId(payload.Data.Author.UserOpenID)
		ctx.SetMessageOrigin(constant.PrivateMessage)
		targetCommand, commandPath := plugin.ResolveCommand(cmd, payload.Data.Content)
		if !cmd.Role.CanUse(payload.Data.Author.Role) {
			permissionDenied(cmd, &ctx)
			return
		}
		if targetCommand != cmd && !targetCommand.Role.CanUse(payload.Data.Author.Role) {
			permissionDenied(targetCommand, &ctx)
			return
		}
		if cmd.DisablePrivate || targetCommand.DisablePrivate {
			permissionDenied(targetCommand, &ctx)
			return
		}
		if !plugin.CanUse(cmd.PluginId, commandPath, payload.Data.Author.UserOpenID, "") {
			permissionDenied(targetCommand, &ctx)
			return
		}

		var parsed any
		if cmd.ParserTarget != nil {
			result := reflect.New(cmd.ParserTarget)
			if err := cmd.Parser.Parse(payload.Data.Content, result.Interface()); err != nil {
				return
			}
			parsed = result.Interface()
		} else {
			var result string
			if err := cmd.Parser.Parse(payload.Data.Content, &result); err != nil {
				return
			}
			parsed = result
		}

		ctx.Parsed = parsed
		go messageRecoveryFunc(cmd, targetCommand, &ctx)
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
		ctx.SetGroupId(payload.Data.GroupOpenID)
		userID := payload.Data.Author.MemberOpenID
		if userID == "" {
			userID = payload.Data.Author.UserOpenID
		}
		if userID == "" {
			userID = payload.Data.Author.UnionID
		}
		ctx.SetUserId(userID)
		go callbackHandleFunc(callbackFunc, ctx)
	case constant.GROUP_JOIN_REQUEST:
		var answer string
		switch payload.Data.VerifyInfo.Method {
		case "verify_message":
			answer = payload.Data.VerifyInfo.VerifyMsg
		case "admin_review_qa":
			if len(payload.Data.VerifyInfo.AnswerList) < 1 {
				// 增加报错
				return
			}
			answer = payload.Data.VerifyInfo.AnswerList[0].Answer
		default:
			return
		}
		ctx := &context.ApplyJoinGroupContext{
			Answer: answer,
		}
		ctx.Init(payload.Data.JoinRequestId, payload.Data.GroupOpenID, payload.Data.Author.UserOpenID, client) // 初始化Context
		err := plugin.CallGlobalJoinGroupHandle(ctx)                                                           // 增加recovery
		if err != nil {
			// 打印
		}
		return
	}
}

func messageRecoveryFunc(cmd, lifecycleCommand *plugin.Command, context *context.MessageContext) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("在执行指令%v (插件: %v)时出现panic: %v", cmd.Prefix, cmd.PluginId, r)
			invokeErrorHook(cmd, lifecycleCommand, context, fmt.Errorf("command panic: %v", r))
		}
	}()
	if err := cmd.Handle(context); err != nil {
		log.Printf("在执行指令%v (插件: %v)时出现error: %v", cmd.Prefix, cmd.PluginId, err)
		invokeErrorHook(cmd, lifecycleCommand, context, err)
	}
}

func invokeErrorHook(cmd, lifecycleCommand *plugin.Command, ctx *context.MessageContext, commandErr error) {
	if lifecycleCommand.HandleError == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("在处理指令%v (插件: %v)的error时出现panic: %v", cmd.Prefix, cmd.PluginId, recovered)
		}
	}()
	if handleErr := lifecycleCommand.HandleError(ctx, commandErr); handleErr != nil {
		log.Printf("在处理指令%v (插件: %v)的error时再次出现error: %v", cmd.Prefix, cmd.PluginId, handleErr)
	}
}

func permissionDenied(cmd *plugin.Command, ctx *context.MessageContext) {
	if cmd.PermissionDenied == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("在执行指令%v (插件: %v)的权限拒绝处理函数时出现panic: %v", cmd.Prefix, cmd.PluginId, recovered)
		}
	}()
	if err := cmd.PermissionDenied(ctx); err != nil {
		log.Printf("在执行指令%v (插件: %v)的权限拒绝处理函数时出现error: %v", cmd.Prefix, cmd.PluginId, err)
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
