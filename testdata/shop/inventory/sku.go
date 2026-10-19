package inventory

import (
	"errors"
	"strings"
)

// ErrBadSKU is returned when a SKU does not look like "ABC-123".
var ErrBadSKU = errors.New("inventory: malformed SKU")

// ParseSKU normalizes a SKU to upper case and checks its format.
//
// Deprecated: use [strings.ToUpper] and validate the format yourself; this
// function will be removed in v2.
func ParseSKU(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 7 || s[3] != '-' {
		return "", ErrBadSKU
	}
	return s, nil
}

// BUG(dan): ParseSKU accepts lowercase letters after the dash.
