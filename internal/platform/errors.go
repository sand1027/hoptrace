package platform

import (
	"errors"
	"fmt"
)

// Exit codes (BSD sysexits-inspired, matching hoptrace).
const (
	ExitOK       = 0
	ExitSLOFail  = 4
	ExitUsage    = 64
	ExitSoftware = 70
	ExitTempFail = 75
)

// ExitError carries a process exit code for CLI handlers (testable without os.Exit).
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func (e *ExitError) Unwrap() error { return e.Err }

func NewExitError(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}

// ExitCodeOf returns a hoptrace exit code for err (default ExitSoftware).
func ExitCodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		return ExitUsage
	}
	return ExitSoftware
}

// UsageError marks invalid user input.
type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string { return e.Msg }

func NewUsageError(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}

// NetworkError marks transient network / TLS failures.
type NetworkError struct {
	Msg string
	Err error
}

func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *NetworkError) Unwrap() error { return e.Err }
