package utils

import "strings"

// 处理多余空白
func NormalizeWhitespace(s string) string {
	// s = strings.Replace(s, "\n", " ", 1)
	fields := strings.Fields(s)
	// 使用单个空格连接切片
	return strings.Join(fields, " ")
}
