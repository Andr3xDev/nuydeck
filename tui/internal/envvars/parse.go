package envvars

import (
	"regexp"
	"strings"
)

// Var is one hl.env call, enabled or commented out.
type Var struct {
	Name  string
	Value string
	// Line is the 0-based line of the call in the file.
	Line int
	// Off marks a call commented out with "--".
	Off bool
}

// Line-anchored on purpose: prose, multi-line calls and non-literal arguments are not listed.
// A documentation comment such as `-- hl.env("X", "y")` cannot be told from a disabled variable and lists as off.
var hlEnvLine = regexp.MustCompile(`^([ \t]*)(--[ \t]*)?hl\.env\([ \t]*"([^"]+)"[ \t]*,[ \t]*"([^"]*)"[ \t]*\)`)

// parseLine parses one line; indentEnd and markEnd are the byte offsets after the indent and after the "--" marker.
func parseLine(line string) (v Var, indentEnd, markEnd int, ok bool) {
	m := hlEnvLine.FindStringSubmatchIndex(line)
	if m == nil {
		return Var{}, 0, 0, false
	}
	v = Var{Name: line[m[6]:m[7]], Value: line[m[8]:m[9]], Off: m[4] >= 0}
	return v, m[3], max(m[3], m[5]), true
}

// ParseHlEnv lists the well-formed hl.env("NAME", "value") lines of text in file order, on and off; nil when none.
func ParseHlEnv(text string) []Var {
	var vars []Var
	for i, line := range strings.Split(text, "\n") {
		if v, _, _, ok := parseLine(line); ok {
			v.Line = i
			vars = append(vars, v)
		}
	}
	return vars
}
