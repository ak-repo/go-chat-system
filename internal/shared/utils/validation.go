package utils

import (
	"regexp"
	"strings"
)

const (
	MinPasswordLength = 8
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func Required(val string) bool {
	return strings.TrimSpace(val) != ""
}

// ValidateEmail checks if the email format is valid
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	return emailRegex.MatchString(email)
}

// ValidatePassword checks password meets minimum requirements
func ValidatePassword(password string) bool {
	if len(password) < MinPasswordLength {
		return false
	}
	return true
}
