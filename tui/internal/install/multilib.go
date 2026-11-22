package install

import (
	"fmt"
	"strings"
)

const (
	multilibHeader  = "[multilib]"
	multilibInclude = "Include"
)

// EnableMultilib uncomments the two-line [multilib] block; an already active block returns changed=false.
func EnableMultilib(content []byte) ([]byte, bool, error) {
	lines := strings.Split(string(content), "\n")
	for i := 0; i+1 < len(lines); i++ {
		header := strings.TrimRight(lines[i], " \t")
		include := strings.TrimRight(lines[i+1], " \t")
		switch {
		case header == multilibHeader && strings.HasPrefix(include, multilibInclude):
			return content, false, nil
		case header == "#"+multilibHeader && strings.HasPrefix(include, "#"+multilibInclude):
			lines[i] = strings.TrimPrefix(header, "#")
			lines[i+1] = strings.TrimPrefix(include, "#")
			return []byte(strings.Join(lines, "\n")), true, nil
		}
	}
	return nil, false, fmt.Errorf("pacman.conf: no [multilib] block with an Include line, edit it by hand")
}
