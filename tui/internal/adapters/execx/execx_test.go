package execx

import (
	"bytes"
	"context"
	"testing"
)

func TestRunExitCodeStderrAndLocale(t *testing.T) {
	var out bytes.Buffer
	res, err := Run(context.Background(), &out, "sh", "-c", `echo $LC_ALL; echo oops >&2; exit 3`)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 || res.Stderr != "oops\n" || out.String() != "C\n" {
		t.Fatalf("got %+v out=%q", res, out.String())
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, nil, "sleep", "5"); err == nil {
		t.Fatal("want error on cancelled context")
	}
}
