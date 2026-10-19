// Package strictjson decodes contract objects without accepting unknown or null fields.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Object decodes a JSON object into the supplied field targets.
func Object(data []byte, required []string, fields map[string]any) error {
	if !utf8.Valid(data) {
		return errors.New("invalid UTF-8")
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return errors.New("expected object")
	}
	for _, name := range required {
		if _, ok := values[name]; !ok {
			return fmt.Errorf("%s: required", name)
		}
	}
	for name, value := range values {
		target, ok := fields[name]
		if !ok {
			return unknownField(name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s: null is not allowed", name)
		}
		if err := json.Unmarshal(value, target); err != nil {
			var typeError *json.UnmarshalTypeError
			var syntaxError *json.SyntaxError
			if errors.As(err, &typeError) || errors.As(err, &syntaxError) {
				return fmt.Errorf("%s: incompatible type or value", name)
			}
			return fmt.Errorf("%s: invalid value: %w", name, err)
		}
	}
	return nil
}

func unknownField(name string) error {
	characters := []rune(name)
	if len(characters) > 20 {
		name = string(characters[:20]) + "..."
	}
	return fmt.Errorf("unknown field %q", name)
}
