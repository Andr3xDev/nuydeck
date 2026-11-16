package editor_test

import (
	"reflect"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/editor"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		hasNano  bool
		want     []string
		wantFail bool
	}{
		{"visual first", map[string]string{"VISUAL": "vim", "EDITOR": "emacs"}, true, []string{"vim"}, false},
		{"editor", map[string]string{"EDITOR": "emacs"}, true, []string{"emacs"}, false},
		{"nano fallback", nil, true, []string{"nano"}, false},
		{"none", nil, false, nil, true},
		{"visual with args", map[string]string{"VISUAL": "code -w"}, false, []string{"code", "-w"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editor.Resolve(
				func(k string) string { return tt.env[k] },
				func(c string) bool { return c == "nano" && tt.hasNano },
			)
			if (err != nil) != tt.wantFail {
				t.Fatalf("err = %v, wantFail %v", err, tt.wantFail)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
