package constant

type EventType string

const (
	INTERACTION_CREATE      EventType = "INTERACTION_CREATE"
	C2C_MESSAGE_CREATE      EventType = "C2C_MESSAGE_CREATE"
	GROUP_AT_MESSAGE_CREATE EventType = "GROUP_AT_MESSAGE_CREATE"
	GROUP_MESSAGE_CREATE    EventType = "GROUP_MESSAGE_CREATE"
	GROUP_JOIN_REQUEST      EventType = "GROUP_JOIN_REQUEST"
)

func IsValidEventType(s string) bool {
	switch EventType(s) {
	case INTERACTION_CREATE, C2C_MESSAGE_CREATE, GROUP_AT_MESSAGE_CREATE, GROUP_MESSAGE_CREATE, GROUP_JOIN_REQUEST:
		return true
	default:
		return false
	}
}
