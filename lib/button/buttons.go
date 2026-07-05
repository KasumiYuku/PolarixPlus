package buttons

import (
	"encoding/json"
	"fmt"
)

type RenderData struct {
	Label   string      `json:"label"`
	Visited string      `json:"visited_label"`
	Style   ButtonStyle `json:"style"`
}

type Permission struct {
	Type           AllowedPermission `json:"type"`
	SpecifyUserIds []string          `json:"specify_user_ids,omitempty"`
}

type Action struct {
	Type          ButtonAction `json:"-"`
	Url           string       `json:"-"` // 跳转目标
	CallbackData  string       `json:"-"` // 回调数据
	Msg           string       `json:"-"` // 自动消息
	Reply         bool         `json:"-"` // 是否携带引用
	AutoSend      bool         `json:"-"` // 是否自动发送指令
	Anchor        bool         `json:"-"` // 唤起图片选择
	Permission    Permission   `json:"-"` // 权限设置
	UnsupportTips string       `json:"-"` // 不支持此功能时的提示
}

type actionJson struct {
	Type          int        `json:"type"`
	Data          string     `json:"data"`
	Reply         bool       `json:"reply,omitempty"`
	Anchor        bool       `json:"anchor,omitempty"`
	UnsupportTips string     `json:"unsupport_tips,omitempty"`
	Permission    Permission `json:"permission"`
}

type Button struct {
	Id         string     `json:"id"`
	RenderData RenderData `json:"render_data"`
	Action     `json:"-"`
	JsonData   actionJson `json:"action"`
}

type Buttons struct {
	List []Button `json:"buttons"`
}

type Keyboard struct {
	Rows []Buttons `json:"rows"`
}

type keyboardJson struct {
	Content Keyboard `json:"content"`
}

func (k *Keyboard) Marshal() ([]byte, error) {
	return GenerateJson(*k)
}

func GenerateJson(keyboard Keyboard) ([]byte, error) {
	if len(keyboard.Rows) == 0 {
		return make([]byte, 0), nil
	}
	if len(keyboard.Rows) > 5 {
		return make([]byte, 0), fmt.Errorf("Rows must less than 5 lines, but this keyboard has %v", len(keyboard.Rows))
	}
	for k, v := range keyboard.Rows {
		if len(v.List) > 5 {
			return make([]byte, 0), fmt.Errorf("Buttons in one row must less than 5, but row %v has %v", k, len(v.List))
		}
	}
	for i := 0; i < len(keyboard.Rows); i++ {
		for j := 0; j < len(keyboard.Rows[i].List); j++ {
			value := &keyboard.Rows[i].List[j] // 取指针

			if value.Type == Callback && value.CallbackData == "" {
				return make([]byte, 0), fmt.Errorf("Button %v need CallbackData when ActionType is Callback", value.Id)
			} else if value.Type == Command && value.Msg == "" {
				return make([]byte, 0), fmt.Errorf("Button %v need Msg when ActionType is Command", value.Id)
			} else if value.Type == Link && value.Url == "" {
				return make([]byte, 0), fmt.Errorf("Button %v need Url when ActionType is Link", value.Id)
			} else if !IsVaildActionType(value.Type) {
				return make([]byte, 0), fmt.Errorf("Button %v define a invailed actionType: %v", value.Id, value.Type)
			}

			value.JsonData = actionJson{}
			value.JsonData.Type = int(value.Type)
			switch value.Type {
			case Callback:
				value.JsonData.Data = value.CallbackData
			case Command:
				value.JsonData.Data = value.Msg
			case Link:
				value.JsonData.Data = value.Url
			}
			value.JsonData.Reply = value.Reply
			value.JsonData.Anchor = value.Anchor
			value.JsonData.UnsupportTips = value.UnsupportTips
			value.JsonData.Permission = value.Permission
		}
	}
	raw := keyboardJson{
		Content: keyboard,
	}
	js, err := json.Marshal(raw)
	if err != nil {
		return make([]byte, 0), err
	}
	return js, nil
}

func (k *Keyboard) AppendButton(id string, label string, visited string, style ButtonStyle, row int) (*Button, error) {
	if row > 4 || row < 0 {
		return nil, fmt.Errorf("Row must between 0 and 5, but received %v", row)
	}
	button := Button{
		RenderData: RenderData{
			Label:   label,
			Visited: visited,
			Style:   style,
		},
		Id: id,
	}
	// 补全长度
	if len(k.Rows) <= row {
		for len(k.Rows) != row+1 {
			k.Rows = append(k.Rows, Buttons{})
		}
	}
	if len(k.Rows[row].List) == 5 {
		return nil, fmt.Errorf("The row %v is full, can't append new button", row)
	}
	k.Rows[row].List = append(k.Rows[row].List, button)
	return &k.Rows[row].List[len(k.Rows[row].List)-1], nil
}

func (button *Button) SetAutoCommand(content string, autoSend bool, anchor bool) *Button {
	button.Action.Anchor = anchor
	button.Action.Msg = content
	button.Action.Type = Command
	button.Action.AutoSend = autoSend
	return button
}

func (button *Button) SetHref(url string) *Button {
	button.Action.Type = Link
	button.Action.Url = url
	return button
}

func (button *Button) SetCallback(data string) *Button {
	button.Action.CallbackData = data
	button.Action.Type = Callback
	return button
}

func (button *Button) SetPermission(required AllowedPermission) *Button {
	button.Permission.Type = required
	return button
}

func (button *Button) SetUserWhiteList(users []string) *Button {
	button.Permission.Type = SomeUser
	button.Permission.SpecifyUserIds = users
	return button
}

func (button *Button) SetUnsupportedTip(tip string) *Button {
	button.Action.UnsupportTips = tip
	return button
}
