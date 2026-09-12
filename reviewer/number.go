package reviewer

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// positiveInt64 accepts JSON Schema integers, including integral decimal and
// exponent forms, without rounding through floating point or expanding an
// unbounded exponent. Its zero value represents an omitted location field.
type positiveInt64 int64

func (n *positiveInt64) UnmarshalJSON(data []byte) error {
	invalid := func() error { return fmt.Errorf("must be a positive signed 64-bit integer") }
	token := string(bytes.TrimSpace(data))
	if token == "" || token[0] < '0' || token[0] > '9' {
		return invalid()
	}
	var exponent int64
	if i := strings.IndexAny(token, "eE"); i >= 0 {
		value, err := strconv.ParseInt(token[i+1:], 10, 64)
		if err != nil {
			return invalid()
		}
		exponent, token = value, token[:i]
	}
	whole, fraction, _ := strings.Cut(token, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return invalid()
	}
	normalized := strings.TrimRight(digits, "0")
	if len(normalized) > 19 {
		return invalid()
	}
	// Removing decimal places and trailing zeroes leaves normalized * 10^scale.
	// A negative scale is fractional; more than 19 digits cannot fit int64.
	requiredExponent := int64(len(fraction) - (len(digits) - len(normalized)))
	if exponent < requiredExponent || exponent > requiredExponent+int64(19-len(normalized)) {
		return invalid()
	}
	scale := int(exponent - requiredExponent)
	value, err := strconv.ParseInt(normalized+strings.Repeat("0", scale), 10, 64)
	if err != nil || value <= 0 {
		return invalid()
	}
	*n = positiveInt64(value)
	return nil
}
