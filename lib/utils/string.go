package utils

import "strings"

// NormalizeWhitespace 压缩连续空白为单个空格。
func NormalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
