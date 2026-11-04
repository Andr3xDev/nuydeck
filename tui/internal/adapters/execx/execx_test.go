package execx

import (
	"bytes"
	"context"
	"testing"
	"time"
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

func TestRunCancelSendsSIGTERM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	_, err := Run(ctx, &out, "sh", "-c", `trap 'echo terminated; exit 0' TERM; while :; do sleep 0.05; done`)
	if err == nil {
		t.Fatal("want error on cancelled context")
	}
	if out.String() != "terminated\n" {
		t.Fatalf("child did not see SIGTERM: out=%q", out.String())
	}
}

func TestRunCancelReturnsWhenChildHoldsStderrPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, nil, "sh", "-c", "sleep 30 & sleep 30")
	if err == nil {
		t.Fatal("want error on cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run hung %v on an orphaned child holding the pipe", elapsed)
	}
}
