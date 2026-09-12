package reviewer

import (
	"reflect"
	"testing"
)

func TestStubReturnsDeterministicEmptyReview(t *testing.T) {
	stub := &Stub{}
	want := Payload{Summary: "Stub review completed", Comments: []Comment{}}
	for _, diff := range []string{"", "arbitrary", "diff --git a/a b/a"} {
		got, err := stub.Review(Request{Diff: diff, Config: Config{Name: "configured", Prompt: "ignored"}})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, %v", got, err)
		}
	}
}
