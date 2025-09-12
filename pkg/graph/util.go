package graph

import (
	"unicode"
)

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if unicode.IsLower(runes[0]) {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}
