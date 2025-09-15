package util

import (
	"math/rand"
	"time"
)

func init() {
	seed := int64(time.Now().UnixNano())
	source := rand.NewSource(seed)

	rand.New(source)
}

// Capitalize first letter (for labels)
func Capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = rune(runes[0] - 32) // assuming lowercase ASCII
	return string(runes)
}
