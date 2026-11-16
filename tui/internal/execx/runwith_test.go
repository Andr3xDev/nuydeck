package execx

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// firstWriteWriter records when the first byte arrives and keeps everything written.
type firstWriteWriter struct {
	mu    sync.Mutex
	buf   strings.Builder
	first time.Time
}

func (w *firstWriteWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.first.IsZero() {
		w.first = time.Now()
	}
	return w.buf.Write(p)
}

func (w *firstWriteWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *firstWriteWriter) firstAt() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.first
}

func TestRunWithStreamsStdoutAndStderrWhileRunning(t *testing.T) {
	const hold = 600 * time.Millisecond
	var out, errw firstWriteWriter
	start := time.Now()

	_, err := RunWith(context.Background(), Options{Stdout: &out, Stderr: &errw}, "sh", "-c", `echo out1; echo err1 >&2; sleep 0.6; echo out2`)
	if err != nil {
		t.Fatal(err)
	}

	if total := time.Since(start); total < hold {
		t.Fatalf("process finished too early (%v)", total)
	}
	for name, w := range map[string]*firstWriteWriter{"stdout": &out, "stderr": &errw} {
		if lag := w.firstAt().Sub(start); lag >= hold {
			t.Errorf("%s first byte arrived after %v, want it before the process ends", name, lag)
		}
	}
	if out.String() != "out1\nout2\n" || errw.String() != "err1\n" {
		t.Errorf("stdout=%q stderr=%q", out.String(), errw.String())
	}
}

func TestRunWithKeepsStderrTailInResultWhileStreaming(t *testing.T) {
	var errw firstWriteWriter

	script := `echo first >&2; head -c ` + strconv.Itoa(2*stderrTailCap) + ` /dev/zero | tr '\0' a >&2; echo last >&2; exit 2`
	res, err := RunWith(context.Background(), Options{Stderr: &errw}, "sh", "-c", script)
	if err != nil {
		t.Fatal(err)
	}

	if res.ExitCode != 2 || !strings.HasSuffix(res.Stderr, "alast\n") {
		t.Errorf("exit %d, stderr tail ends %q, want exit 2 ending with the last line", res.ExitCode, res.Stderr[max(0, len(res.Stderr)-12):])
	}
	if len(res.Stderr) != stderrTailCap || strings.Contains(res.Stderr, "first") {
		t.Errorf("kept %d bytes (contains start: %v), want exactly the last %d", len(res.Stderr), strings.Contains(res.Stderr, "first"), stderrTailCap)
	}
	if !strings.HasPrefix(errw.String(), "first\n") || !strings.HasSuffix(errw.String(), "last\n") {
		t.Error("streamed stderr must stay complete")
	}
}

func TestRunWithRunsInDir(t *testing.T) {
	dir := t.TempDir()
	var out firstWriteWriter

	if _, err := RunWith(context.Background(), Options{Stdout: &out, Dir: dir}, "pwd", "-P"); err != nil {
		t.Fatal(err)
	}

	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != want {
		t.Errorf("cwd = %q, want %q", got, want)
	}
}

// Children must stay in the TUI's session: sudo's tty timestamp is per terminal,
// so a Setsid child would not see the credentials cached by the handoff (design D19).
func TestRunWithChildKeepsParentSession(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("ps not available")
	}
	// syscall has no Getsid wrapper; pid 0 means the calling process.
	parent, _, errno := syscall.RawSyscall(syscall.SYS_GETSID, 0, 0, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	var out firstWriteWriter

	if _, err := RunWith(context.Background(), Options{Stdout: &out}, "sh", "-c", `ps -o sid= -p $$`); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(out.String()); got != strconv.Itoa(int(parent)) {
		t.Errorf("child session = %q, want parent's %d", got, parent)
	}
}
