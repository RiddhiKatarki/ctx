package clierr

import "fmt"

// Exit codes used by the CLI.
// Stable contract: tooling (VSCode ext, scripts) may rely on these.
const (
	ExitSuccess = 0
	ExitUser    = 1 // generic user error (bad input, missing arg)
	ExitSystem  = 2 // system error (git unavailable, IO failure)
	ExitBundle  = 3 // invalid .ctx bundle
)

// Error codes — stable, machine-readable strings that identify
// the class of failure for tooling consumers.
const (
	CodeUser   = "user_error"
	CodeSystem = "system_error"
	CodeBundle = "invalid_bundle"
)

// Error is a structured CLI error. It carries a stable code and
// message that can be surfaced via JSON when the --json flag is set,
// and an integer exit code for the process.
type Error struct {
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap allows errors.Is / errors.As to traverse the cause chain.
func (e *Error) Unwrap() error {
	return e.Cause
}

// ExitCode returns the process exit code corresponding to this error.
func (e *Error) ExitCode() int {
	switch e.Code {
	case CodeUser:
		return ExitUser
	case CodeSystem:
		return ExitSystem
	case CodeBundle:
		return ExitBundle
	default:
		return ExitUser
	}
}

// New constructs a *Error with no cause.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap constructs a *Error that wraps an underlying cause.
func Wrap(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}
