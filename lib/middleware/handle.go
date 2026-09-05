package middleware

import (
	"Plrx/lib/buttons"
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/logx"
	"Plrx/lib/message"
	"Plrx/lib/parser"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/state"
	"Plrx/lib/structers"
	"Plrx/lib/utils"
	"fmt"
	"strings"
)

var messageLog = logx.New("message")

// commandDispatchOpts 群消息与私聊两条分发路径的差异参数
type commandDispatchOpts struct {
	userID  string                 // 发送者 openid
	groupID string                 // 群 openid（私聊为空）
	origin  constant.MessageOrigin // 发送目标
	raw     string                 // 未清洗的原始消息
	// DisablePrivate 检查仅私聊生效，群消息跳过
	checkPrivate bool
}

// dispatchCommand 指令解析与分发: 三态智能匹配 + 树路由 + kong 参数解析。
func dispatchCommand(payload structers.Payload, client *qqapi.Client, opts commandDispatchOpts) {
	tokens := strings.Fields(payload.Data.Content)
	if len(tokens) == 0 {
		return
	}
	root, afterRoot, ok := plugin.ResolveRoot(tokens)
	if !ok {
		return
	}
	if !plugin.Enabled(root.PluginId) {
		return
	}
	leaf, commandPath, rest := plugin.Resolve(root, afterRoot)

	ctx := &context.MessageContext{
		UserMessage: message.UserMessage{
			Content:     payload.Data.Content,
			Attachments: payload.Data.Attachments,
		},
		Raw: opts.raw,
	}
	ctx.Init(payload.Data.Id, payload.ID, client)
	ctx.BindStorage(leaf.PluginId, commandPath)
	if opts.groupID != "" {
		ctx.SetGroupId(opts.groupID)
	}
	ctx.SetUserId(opts.userID)
	ctx.SetMessageOrigin(opts.origin)

	if !root.Role.CanUse(payload.Data.Author.Role) {
		messageLog.Warnf("用户%v无权限使用%v指令", payload.Data.Author.Username, root.Prefix)
		permissionDenied(root, ctx)
		return
	}
	if leaf != root && !leaf.Role.CanUse(payload.Data.Author.Role) {
		messageLog.Warnf("用户%v无权限使用%v指令", payload.Data.Author.Username, commandPath)
		permissionDenied(leaf, ctx)
		return
	}
	if opts.checkPrivate && (root.DisablePrivate || leaf.DisablePrivate) {
		permissionDenied(leaf, ctx)
		return
	}
	if !plugin.CanUse(root.PluginId, commandPath, opts.userID, opts.groupID) {
		permissionDenied(leaf, ctx)
		return
	}

	if leaf.Args != nil {
		// 声明式参数: kong 解析剩余词元, 失败回复用法
		parsed, usage, err := parser.ParseArgs(leaf.Prefix, leaf.Args, rest)
		if err != nil {
			reply := fmt.Sprintf("指令参数有误: %v", err)
			if usage != "" {
				reply += "\n" + usage
			}
			if err := ctx.Text(reply).Send(); err != nil {
				messageLog.Errorf("发送用法提示失败: %v", err)
			}
			return
		}
		ctx.Parsed = parsed
	} else {
		// 快速通道: 剩余文本, 零反射
		ctx.Parsed = strings.Join(rest, " ")
	}
	pool.Go(func() { messageRecoveryFunc(root, leaf, ctx) })
}

func ProcessPayload(payload structers.Payload, client *qqapi.Client) {
	switch payload.EventType {
	case constant.GROUP_AT_MESSAGE_CREATE, constant.GROUP_MESSAGE_CREATE:
		state.IncRecv()
		raw := payload.Data.Content
		payload.Data.Content = utils.FilterAt(payload.Data.Content)
		userID := payload.Data.Author.MemberOpenID
		if userID == "" {
			userID = payload.Data.Author.UnionID
		}
		dispatchCommand(payload, client, commandDispatchOpts{
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
