package sentinal_errors

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInvalidInput       = errors.New("invalid input")
	ErrTooLarge           = errors.New("file too large")
	ErrRateLimited        = errors.New("rate limited")
	ErrQueueFull          = errors.New("queue full")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrAlreadyExists      = errors.New("already exists")
	ErrNotUploaded        = errors.New("file not uploaded")
)


// NowPtr returns a pointer to current time
func NowPtr() *time.Time {
	now := time.Now()
	return &now
}
