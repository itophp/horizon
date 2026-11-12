package contracts

import "errors"

// Error carries a stable service code across every layer. Codes are part of the
// public contract: adapters, agents and UI all branch on them.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

// Problem builds a coded error.
func Problem(code string) error { return &Error{Code: code} }

// Code extracts the stable code, defaulting to internal.
func Code(err error) string {
	var coded *Error
	if err != nil && errors.As(err, &coded) {
		return coded.Code
	}
	return "internal"
}
