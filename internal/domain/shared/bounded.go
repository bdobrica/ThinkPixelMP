// Package shared contains dependency-free value objects shared by domain packages.
package shared

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxBoundedStringBytes = 4096

// BoundedString is validated UTF-8 text with an explicit byte limit.
type BoundedString struct{ value string }

func NewBoundedString(value string, maxBytes int) (BoundedString, error) {
	if maxBytes < 1 || maxBytes > MaxBoundedStringBytes {
		return BoundedString{}, fmt.Errorf("bounded string: invalid maximum")
	}
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) {
		return BoundedString{}, fmt.Errorf("bounded string: invalid length or encoding")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return BoundedString{}, fmt.Errorf("bounded string: control character")
		}
	}
	return BoundedString{value: value}, nil
}
func (value BoundedString) String() string { return value.value }

// ReasonCode is a stable, telemetry-safe lowercase code.
type ReasonCode struct{ value string }

func NewReasonCode(value string) (ReasonCode, error) {
	if len(value) < 1 || len(value) > 128 || strings.TrimSpace(value) != value {
		return ReasonCode{}, fmt.Errorf("reason code: invalid length")
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if index > 0 && index < len(value)-1 {
			valid = valid || character == '.' || character == '_' || character == '-'
		}
		if !valid {
			return ReasonCode{}, fmt.Errorf("reason code: invalid character")
		}
	}
	return ReasonCode{value: value}, nil
}
func (code ReasonCode) String() string { return code.value }
