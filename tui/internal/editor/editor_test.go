package editor_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/editor"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		installed []string
		want      []string
		wantFail  bool
	}{
		{"visual first", map[string]string{"VISUAL": "vim", "EDITOR": "emacs"}, []string{"nano"}, []string{"vim"}, false},
		{"editor", map[string]string{"EDITOR": "emacs"}, []string{"nano"}, []string{"emacs"}, false},
		{"nvim wins over the rest", nil, []string{"vi", "nano", "vim", "nvim"}, []string{"nvim"}, false},
		{"vim when no nvim", nil, []string{"vi", "nano", "vim"}, []string{"vim"}, false},
		{"nano when no nvim or vim", nil, []string{"vi", "nano"}, []string{"nano"}, false},
		{"vi last", nil, []string{"vi"}, []string{"vi"}, false},
		{"none", nil, nil, nil, true},
		{"visual with args", map[string]string{"VISUAL": "code -w"}, nil, []string{"code", "-w"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editor.Resolve(
				func(k string) string { return tt.env[k] },
				func(c string) bool { return slices.Contains(tt.installed, c) },
			)
			if (err != nil) != tt.wantFail {
				t.Fatalf("err = %v, wantFail %v", err, tt.wantFail)
			}
			if tt.wantFail && err.Error() != "no editor: set $VISUAL or $EDITOR or install nvim, vim, nano or vi" {
				t.Fatalf("err = %q, want the four-editor message", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
