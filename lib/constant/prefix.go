package constant

import "sync/atomic"

// 指令前缀符号集, 启动时由配置写入; "" 表示允许无前缀调用。
var prefixChars atomic.Value // []string

func DefaultPrefixChars() []string { return []string{"/", "#", ""} }

func SetPrefixChars(s []string) {
	if len(s) == 0 {
		s = DefaultPrefixChars()
	}
	prefixChars.Store(s)
}

func PrefixChars() []string {
	if v, ok := prefixChars.Load().([]string); ok {
		return v
	}
	return DefaultPrefixChars()
}

// HasBarePrefix 是否允许不带符号的裸指令名。
func HasBarePrefix() bool {
	for _, p := range PrefixChars() {
		if p == "" {
			return true
		}
	}
	return false
}
