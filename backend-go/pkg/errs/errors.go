package errs

import (
	"errors"
	"fmt"
)

// Common (global) errors
var (
	ErrInternal     = errors.New("internal error")
	ErrNotFound     = errors.New("resource not found")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation failed")
)

// Friend module errors
var (
	ErrSelfAction          = errors.New("cannot perform this action on yourself")
	ErrRequestNotFound     = errors.New("friend request not found")
	ErrAlreadyFriends      = errors.New("users are already friends")
	ErrBlockedRelationship = errors.New("one of the users has blocked the other")
	ErrBlockNotFound       = errors.New("block relationship not found")
)

//
// Error wrapping helpers (common patterns)
//

// Wrap adds context while preserving the original error for errors.Is / errors.As
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

// New creates a formatted error (use for internal-only errors, not domain errors)
func New(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// Is is just a shortcut (optional)
func Is(err, target error) bool {
	return errors.Is(err, target)
}
