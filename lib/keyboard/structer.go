package keyboard

import (
	"Plrx/lib/contract"
	"encoding/json"
)

type Keyboard struct {
}

func (keyboard Keyboard) Marshal() ([]byte, error) {
	return json.Marshal(keyboard)
}

// 验证接口实现
var _ contract.CanMarshal = &Keyboard{}
