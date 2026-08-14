package platform

import "fmt"

// Exit codes (BSD sysexits-inspired, matching hoptrace).
const (
	ExitOK       = 0
	ExitSLOFail  = 4
	ExitUsage    = 64
	ExitSoftware = 70
	ExitTempFail = 75
)

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
