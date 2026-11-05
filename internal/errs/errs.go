// Package errs defines the shared error type with stable codes and exit codes.
package errs

import "errors"

const (
	NotFound      = "NOT_FOUND"
	BadInput      = "BAD_INPUT"
	Unsupported   = "UNSUPPORTED"
	TooLong       = "TOO_LONG"
	Exists        = "EXISTS"
	Conflict      = "CONFLICT"
	ConflictRev   = "CONFLICT_REV"
	GitMissing    = "GIT_MISSING"
	ProviderError = "PROVIDER_ERROR"
	LintFailed    = "LINT_FAILED"
)

// Error is a coded error with an optional hint and extra JSON members.
type Error struct {
	Code, Message, Hint string
	Exit                int
	Extra               map[string]any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New returns an error whose exit code derives from its code.
func New(code, message, hint string) *Error {
	return &Error{Code: code, Message: message, Hint: hint}
}

// Fatal is like New but forces exit code 1 (config/fatal errors).
func Fatal(code, message, hint string) *Error {
	return &Error{Code: code, Message: message, Hint: hint, Exit: 1}
}

// CodeOf returns the code of err, or "" when it is not an *Error.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// ExitCode maps an error to a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if !errors.As(err, &e) {
		return 1
	}
	if e.Exit != 0 {
		return e.Exit
	}
	switch e.Code {
	case Conflict, ConflictRev, Exists:
		return 4
	case NotFound, BadInput, Unsupported, TooLong:
		return 2
	case LintFailed:
		return 3
	}
	return 1
}
