package utils

import "strings"

func FilterAt(raw string) string {
	raw = NormalizeWhitespace(raw)
	args := strings.Split(raw, " ")
	if len(args) < 2 {
		return raw
	}
	if strings.HasPrefix(args[0], "<@") || strings.HasPrefix(args[0], "@") {
		return strings.Join(args[1:], " ")
	} else {
		return raw
	}
}
