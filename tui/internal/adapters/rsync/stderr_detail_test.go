package rsync

import (
	"strings"
	"testing"
)

func TestStderrDetail_HintsRsyncVersionOnUnknownOption(t *testing.T) {
	for _, stderr := range []string{"rsync: --mkpath: unknown option\n", "rsync: unrecognized option '--mkpath'\n"} {
		if got := stderrDetail(stderr); !strings.Contains(got, "rsync >= 3.2.3 required (--mkpath)") {
			t.Fatalf("no hint in %q", got)
		}
	}
}

func TestStderrDetail_PlainErrorHasNoHintAndIsOneLine(t *testing.T) {
	if got := stderrDetail("rsync: failed\n  to read\n"); got != "rsync: failed to read" {
		t.Fatalf("got %q", got)
	}
}
