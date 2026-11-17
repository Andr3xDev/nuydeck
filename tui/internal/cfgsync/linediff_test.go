package cfgsync

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDiffLines_Table(t *testing.T) {
	cases := []struct {
		name     string
		dst, src []string
		want     []DiffLine
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"},
			[]DiffLine{{'=', "a"}, {'=', "b"}}},
		{"insert", []string{"a", "c"}, []string{"a", "b", "c"},
			[]DiffLine{{'=', "a"}, {'+', "b"}, {'=', "c"}}},
		{"delete", []string{"a", "b", "c"}, []string{"a", "c"},
			[]DiffLine{{'=', "a"}, {'-', "b"}, {'=', "c"}}},
		{"replace", []string{"a", "x", "c"}, []string{"a", "y", "c"},
			[]DiffLine{{'=', "a"}, {'-', "x"}, {'+', "y"}, {'=', "c"}}},
		{"both empty", nil, nil, nil},
		{"destination empty", nil, []string{"a"}, []DiffLine{{'+', "a"}}},
		{"source empty", []string{"a"}, nil, []DiffLine{{'-', "a"}}},
		{"interleaved keeps the longest common subsequence", []string{"a", "x", "b", "y", "c"}, []string{"a", "b", "z", "c"},
			[]DiffLine{{'=', "a"}, {'-', "x"}, {'=', "b"}, {'-', "y"}, {'+', "z"}, {'=', "c"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DiffLines(c.dst, c.src)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("DiffLines = %v, want %v", got, c.want)
			}
		})
	}
}

// Above maxDiffCells the middle is not aligned: one removed block, then one added block.
func TestDiffLines_AboveCapShowsOneRemovedAndOneAddedBlock(t *testing.T) {
	const side = 2100 // side*side > maxDiffCells (4,000,000)
	dst := make([]string, side)
	src := make([]string, side)
	for i := range dst {
		dst[i] = fmt.Sprintf("d%d", i)
		src[i] = fmt.Sprintf("s%d", i)
	}
	dst = append([]string{"head"}, append(dst, "tail")...)
	src = append([]string{"head"}, append(src, "tail")...)

	got := DiffLines(dst, src)

	var ops strings.Builder
	for _, l := range got {
		ops.WriteRune(l.Op)
	}
	want := "=" + strings.Repeat("-", side) + strings.Repeat("+", side) + "="
	if ops.String() != want {
		t.Errorf("op sequence differs from head, %d removed, %d added, tail (len %d, want %d)", side, side, len(got), len(want))
	}
}
