package envvars

import "regexp"

// Var is one hl.env call.
type Var struct {
	Name  string
	Value string
}

// Line-anchored on purpose: comments, multi-line calls and non-literal arguments are not listed.
var hlEnvLine = regexp.MustCompile(`(?m)^[ \t]*hl\.env\([ \t]*"([^"]+)"[ \t]*,[ \t]*"([^"]*)"[ \t]*\)`)

// ParseHlEnv lists the well-formed hl.env("NAME", "value") lines of text in order; nil when none.
func ParseHlEnv(text string) []Var {
	var vars []Var
	for _, m := range hlEnvLine.FindAllStringSubmatch(text, -1) {
		vars = append(vars, Var{Name: m[1], Value: m[2]})
	}
	return vars
}
