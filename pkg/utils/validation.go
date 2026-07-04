package utils

import "strings"

func Required(val string) bool {
	return strings.TrimSpace(val) != ""
}
