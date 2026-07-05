package message

import (
	"sync"
)

// 计数器
type MsgRef struct {
	msgSeq uint8
	lock   sync.Mutex
}

func (ref *MsgRef) Count() (uint8, error) {
	ref.lock.Lock()
	defer ref.lock.Unlock()
	if ref.msgSeq == 5 {
		return 0, &ReplyMessageReachLimit{}
	}
	ref.msgSeq++
	return ref.msgSeq - 1, nil
}
