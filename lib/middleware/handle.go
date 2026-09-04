package middleware

import (
	"Plrx/lib/buttons"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/logx"
	"Plrx/lib/message"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/state"
	"Plrx/lib/structers"
	"Plrx/lib/utils"
	"fmt"
	"reflect"
	"strings"
)

var messageLog = logx.New("message")

// commandDispatchOpts 群消息与私聊两条分发路径的差异参数
type commandDispatchOpts struct {
	prefix  string                 // 指令前缀
	userID  string                 // 发送者 openid
	groupID string                 // 群 openid（私聊为空）
	origin  constant.MessageOrigin // 发送目标
	raw     string                 // 未清洗的原始消息
	// DisablePrivate 检查仅私聊生效，群消息跳过
	checkPrivate bool
}

// dispatchCommand 从消息指令分发到对应插件处理器（群与私聊共用一条执行链）
func dispatchCommand(payload structers.Payload, client *qqapi.Client, opts commandDispatchOpts) {
	cmd, ok := plugin.GetCommand(opts.prefix)
	if !ok {
		return
	}
	ctx := &context.MessageContext{
		UserMessage: message.UserMessage{
			Content:     payload.Data.Content,
			Attachments: payload.Data.Attachments,
		},
		Raw: opts.raw,
	}
	ctx.Init(payload.Data.Id, payload.ID, client)
	ctx.BindStorage(cmd.PluginId, cmd.Prefix)
	if opts.groupID != "" {
		ctx.SetGroupId(opts.groupID)
	}
	ctx.SetUserId(opts.userID)
	ctx.SetMessageOrigin(opts.origin)

	targetCommand, commandPath := plugin.ResolveCommand(cmd, payload.Data.Content)
	if !cmd.Role.CanUse(payload.Data.Author.Role) {
		messageLog.Warnf("用户%v无权限使用%v指令", payload.Data.Author.Username, cmd.Prefix)
		permissionDenied(cmd, ctx)
		return
	}
	if targetCommand != cmd && !targetCommand.Role.CanUse(payload.Data.Author.Role) {
		messageLog.Warnf("用户%v无权限使用%v指令", payload.Data.Author.Username, commandPath)
		permissionDenied(targetCommand, ctx)
		return
	}
	if opts.checkPrivate && (cmd.DisablePrivate || targetCommand.DisablePrivate) {
		permissionDenied(targetCommand, ctx)
		return
	}
	if !plugin.CanUse(cmd.PluginId, commandPath, opts.userID, opts.groupID) {
		permissionDenied(targetCommand, ctx)
		return
	}

	// 解析器：优先反射到 ParserTarget 结构，否则落到 string
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
	pool.Go(func() { messageRecoveryFunc(cmd, targetCommand, ctx) })
}

func ProcessPayload(payload structers.Payload, client *qqapi.Client) {
	switch payload.EventType {
	case constant.GROUP_AT_MESSAGE_CREATE, constant.GROUP_MESSAGE_CREATE:
		state.IncRecv()
		raw := payload.Data.Content
		payload.Data.Content = utils.FilterAt(payload.Data.Content)
		msgs := strings.Split(payload.Data.Content, " ")
		prefix := msgs[0]
		// @ 机器人场景：第一个 token 是 <@openid>，指令实际从第二个开始
		if len(msgs) > 1 && (strings.HasPrefix(msgs[0], "\u003c@") || strings.HasPrefix(msgs[0], "<@")) && strings.HasSuffix(msgs[0], ">") {
			prefix = msgs[1]
		}
		userID := payload.Data.Author.MemberOpenID
		if userID == "" {
			userID = payload.Data.Author.UnionID
		}
		dispatchCommand(payload, client, commandDispatchOpts{
			prefix:  prefix,
			userID:  userID,
			groupID: payload.Data.GroupOpenID,
			origin:  constant.GroupMessage,
			raw:     raw,
		})
	case constant.C2C_MESSAGE_CREATE:
		state.IncRecv()
		raw := payload.Data.Content
		payload.Data.Content = strings.TrimSpace(payload.Data.Content)
		if payload.Data.Content == "" {
			return
		}
		dispatchCommand(payload, client, commandDispatchOpts{
			prefix:       strings.Fields(payload.Data.Content)[0],
			userID:       payload.Data.Author.UserOpenID,
			origin:       constant.PrivateMessage,
			raw:          raw,
			checkPrivate: true,
		})
	case constant.INTERACTION_CREATE:
		state.IncButton()
		data := payload.Data.Callback.Resolved.ButtonData
		buttonId := payload.Data.Callback.Resolved.ButtonId
		ctx := &context.CallbackContext{}
		ctx.Init(payload.ID, client)
		ctx.InteractionID = payload.Data.Id
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
		// 无论按钮是否已注册，事件到达就先把交互回执掉，避免按钮转圈到超时。
		if err := ctx.Done(); err != nil {
			messageLog.Errorf("回执按钮 %v 失败: %v", buttonId, err)
		}
		callbackFunc, ok := buttons.GetCallbackFunc(buttonId)
		if !ok {
			messageLog.Infof("回调按钮: %v未注册回调函数, 已回执交互", buttonId)
			return
		}
		pool.Go(func() { callbackHandleFunc(callbackFunc, ctx) })
	case constant.GROUP_JOIN_REQUEST:
		var answer string
		switch payload.Data.VerifyInfo.Method {
		case "verify_message":
			answer = payload.Data.VerifyInfo.VerifyMsg
		case "admin_review_qa":
			if len(payload.Data.VerifyInfo.AnswerList) < 1 {
				return
			}
			answer = payload.Data.VerifyInfo.AnswerList[0].Answer
		default:
			return
		}
		ctx := &context.ApplyJoinGroupContext{
			Answer: answer,
		}
		ctx.Init(payload.Data.JoinRequestId, payload.Data.GroupOpenID, payload.Data.Author.UserOpenID, client)
		err := plugin.CallGlobalJoinGroupHandle(ctx)
		if err != nil {
			// 入群申请处理失败仅记录，不中断分发
		}
		return
	case constant.MESSAGE_AUDIT_PASS, constant.MESSAGE_AUDIT_REJECT:
		// 消息审计结果：resolve 等待中的发送方
		qqapi.ResolveAudit(payload.Data.AuditID, payload.Data.MessageId, payload.EventType == constant.MESSAGE_AUDIT_PASS)
		return
	}
}

func messageRecoveryFunc(cmd, lifecycleCommand *plugin.Command, context *context.MessageContext) {
	defer func() {
		if r := recover(); r != nil {
			messageLog.Errorf("在执行指令%v (插件: %v)时出现panic: %v", cmd.Prefix, cmd.PluginId, r)
			invokeErrorHook(cmd, lifecycleCommand, context, fmt.Errorf("command panic: %v", r))
		}
	}()
	if err := cmd.Handle(context); err != nil {
		messageLog.Errorf("在执行指令%v (插件: %v)时出现error: %v", cmd.Prefix, cmd.PluginId, err)
		invokeErrorHook(cmd, lifecycleCommand, context, err)
	}
}

func invokeErrorHook(cmd, lifecycleCommand *plugin.Command, ctx *context.MessageContext, commandErr error) {
	if lifecycleCommand.HandleError == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			messageLog.Errorf("在处理指令%v (插件: %v)的error时出现panic: %v", cmd.Prefix, cmd.PluginId, recovered)
		}
	}()
	if handleErr := lifecycleCommand.HandleError(ctx, commandErr); handleErr != nil {
		messageLog.Errorf("在处理指令%v (插件: %v)的error时再次出现error: %v", cmd.Prefix, cmd.PluginId, handleErr)
	}
}

func permissionDenied(cmd *plugin.Command, ctx *context.MessageContext) {
	if cmd.PermissionDenied == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			messageLog.Errorf("在执行指令%v (插件: %v)的权限拒绝处理函数时出现panic: %v", cmd.Prefix, cmd.PluginId, recovered)
		}
	}()
	if err := cmd.PermissionDenied(ctx); err != nil {
		messageLog.Errorf("在执行指令%v (插件: %v)的权限拒绝处理函数时出现error: %v", cmd.Prefix, cmd.PluginId, err)
	}
}

func callbackHandleFunc(handle buttons.CallbackButtonHandleFunc, ctx *context.CallbackContext) {
	defer func() {
		if r := recover(); r != nil {
			messageLog.Errorf("在执行回调按钮: %v 处理函数时候出现panic: %v", ctx.ButtonId, r)
		}
	}()
	// 交互回执已在事件分发时完成，这里只跑业务，避免重复回执。
	if err := handle(ctx); err != nil {
		messageLog.Errorf("在执行回调按钮: %v 处理函数时候出现error: %v", ctx.ButtonId, err)
	}
}
