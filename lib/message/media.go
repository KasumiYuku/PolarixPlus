package message

import "encoding/json"

type MediaMessage struct {
	*Message
	Media MediaContent `json:"media"`
}

type MediaContent struct {
	FileInfo string `json:"file_info"`
}

func (msg *MediaMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *MediaMessage) Init() {
	if msg.Message == nil {
		msg.Message = (&Message{}).InitRef()
	}
	msg.Message.MarshalInterface = msg
}
