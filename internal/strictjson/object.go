// Package strictjson decodes contract objects without accepting unknown or null fields.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

func Object(data []byte, required []string, fields map[string]any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid UTF-8")
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return fmt.Errorf("expected object")
	}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("%s: required", key)
		}
	}
	for key, value := range values {
		target, ok := fields[key]
		if !ok {
			return fmt.Errorf("unknown field")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s: null is not allowed", key)
		}
		if err := json.Unmarshal(value, target); err != nil {
			// encoding/json includes the complete numeric token in type errors.
			// Report the rule without reflecting untrusted payload data.
			var typeError *json.UnmarshalTypeError
			var syntaxError *json.SyntaxError
			if errors.As(err, &typeError) || errors.As(err, &syntaxError) {
				return fmt.Errorf("%s: incompatible type or value", key)
			}
			return fmt.Errorf("%s: invalid value: %w", key, err)
		}
	}
	return nil
}
