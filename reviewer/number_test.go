package reviewer

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestLocationIntegerJSONForms(t *testing.T) {
	valid := map[string]int64{
		"1": 1, "1.0": 1, "1e0": 1, "1E+1": 10,
		"0.1e1": 1, "100e-2": 1, "12.300e1": 123,
		"9007199254740993.0":                         9007199254740993,
		"9223372036854775807":                        math.MaxInt64,
		"9223372036854775807.000":                    math.MaxInt64,
		"9.223372036854775807e18":                    math.MaxInt64,
		"922337203685477580700e-2":                   math.MaxInt64,
		"1" + strings.Repeat("0", 10000) + "e-10000": 1,
	}
	for raw, want := range valid {
		for _, field := range []string{"line", "end_line"} {
			baseLine := ""
			if field == "end_line" {
				baseLine = `,"line":1`
			}
			data := fmt.Sprintf(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":"new",%q:%s%s}]}`, field, raw, baseLine)
			var p Payload
			if err := json.Unmarshal([]byte(data), &p); err != nil {
				t.Fatalf("valid %s token of length %d: %v", field, len(raw), err)
			}
			if err := ValidatePayload(p); err != nil {
				t.Fatal(err)
			}
			got := p.Comments[0].Line
			if field == "end_line" {
				got = p.Comments[0].EndLine
			}
			if got != want {
				t.Fatalf("decoded %s = %d, want %d", field, got, want)
			}
		}
	}
}

func TestLocationIntegerRejectsWithoutRoundingOrEchoingTokens(t *testing.T) {
	invalid := []string{
		"0", "0.0", "0e10", "-1", "-1.0", "1.5", "1e-1", "10.01",
		"9223372036854775808", "9223372036854775808.0",
		"9223372036854775806.999999999999999999", "9.223372036854775808e18",
		"1e9223372036854775807", "1e-9223372036854775808",
		"1e" + strings.Repeat("9", 10000), "1e-" + strings.Repeat("9", 10000),
		strings.Repeat("9", 10000), `"1"`, "true", "[]", "{}",
	}
	for _, raw := range invalid {
		for _, field := range []string{"line", "end_line"} {
			data := fmt.Sprintf(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":"new",%q:%s}]}`, field, raw)
			var p Payload
			err := json.Unmarshal([]byte(data), &p)
			if err == nil {
				t.Fatalf("accepted invalid %s token of length %d", field, len(raw))
			}
			if len(err.Error()) > 200 || !strings.Contains(err.Error(), "comment[0]") || !strings.Contains(err.Error(), field) {
				t.Fatalf("unbounded or unhelpful diagnostic: length %d", len(err.Error()))
			}
		}
	}
}

func TestWrongFieldTypeDiagnosticsAreBounded(t *testing.T) {
	longNumber := strings.Repeat("7", 10000)
	for _, data := range []string{
		`{"summary":` + longNumber + `,"comments":[]}`,
		`{"summary":"s","comments":[{"file":"a","severity":` + longNumber + `,"message":"m"}]}`,
		`{"summary":"s","comments":[{"file":"a","severity":"low","message":` + longNumber + `}]}`,
	} {
		var p Payload
		err := json.Unmarshal([]byte(data), &p)
		if err == nil || len(err.Error()) > 200 || strings.Contains(err.Error(), longNumber[:100]) {
			t.Fatalf("wrong-field diagnostic not bounded; rejected=%v", err != nil)
		}
	}
}
