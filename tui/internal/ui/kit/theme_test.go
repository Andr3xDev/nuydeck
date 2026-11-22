package kit

import "testing"

func TestForVariant(t *testing.T) {
	cases := map[string]bool{
		"zenit":  false,
		"nadir":  true,
		"":       true,
		"bogus":  true,
		"Zenit ": true,
	}
	for variant, wantDark := range cases {
		if got := ForVariant(variant).Dark; got != wantDark {
			t.Errorf("ForVariant(%q).Dark = %v, want %v", variant, got, wantDark)
		}
	}
}
