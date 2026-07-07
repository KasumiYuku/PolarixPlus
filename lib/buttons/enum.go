package buttons

type AllowedPermission int

const (
	SomeUser AllowedPermission = iota
	Admin
	AllUser
)

type ButtonAction int

const (
	Link ButtonAction = iota
	Callback
	Command
)

func IsVaildActionType(actionType ButtonAction) bool {
	return int(Link) >= 0 && int(actionType) <= int(Command)
}

type ButtonStyle int

const (
	Gray ButtonStyle = iota
	Blue
)
