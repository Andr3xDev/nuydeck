package tomlx

import (
	"reflect"
	"testing"
)

func TestHeaderLinesFindsOnlyTheRequestedTable(t *testing.T) {
	data := []byte("version = 1\n\n[[package]]\nname = \"a\"\n\n[[entry]]\npath = \"x\"\n\n  [[ package ]]\nname = \"b\"\n")

	if got, want := headerLines(data, "package"), []int{3, 9}; !reflect.DeepEqual(got, want) {
		t.Errorf("package lines = %v, want %v", got, want)
	}
	if got, want := headerLines(data, "entry"), []int{6}; !reflect.DeepEqual(got, want) {
		t.Errorf("entry lines = %v, want %v", got, want)
	}
}
