package constant

type MemberRole uint8

const (
	SomeUser MemberRole = iota
	AdminUser
	AllUser
)
