package shared

import "fmt"

type ErrorClass string

const (
	ErrorInvalid      ErrorClass = "invalid"
	ErrorNotFound     ErrorClass = "not_found"
	ErrorConflict     ErrorClass = "conflict"
	ErrorUnauthorized ErrorClass = "unauthorized"
	ErrorForbidden    ErrorClass = "forbidden"
	ErrorUnavailable  ErrorClass = "unavailable"
	ErrorInternal     ErrorClass = "internal"
)

// TypedError carries only a stable class and reason code across boundaries.
type TypedError struct {
	class ErrorClass
	code  ReasonCode
}

func NewTypedError(class ErrorClass, code ReasonCode) error {
	switch class {
	case ErrorInvalid, ErrorNotFound, ErrorConflict, ErrorUnauthorized, ErrorForbidden, ErrorUnavailable, ErrorInternal:
	default:
		return fmt.Errorf("typed error: invalid class")
	}
	if code.String() == "" {
		return fmt.Errorf("typed error: reason code required")
	}
	return &TypedError{class: class, code: code}
}
func (problem *TypedError) Error() string     { return string(problem.class) + ":" + problem.code.String() }
func (problem *TypedError) Class() ErrorClass { return problem.class }
func (problem *TypedError) Code() ReasonCode  { return problem.code }
