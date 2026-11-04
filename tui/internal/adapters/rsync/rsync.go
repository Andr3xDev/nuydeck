package rsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Item is one parsed itemize line: a create, update or delete.
type Item struct {
	Path string
	Type domain.ChangeType
	Kind domain.NodeKind
}

// Request describes one rsync run for a resolved entry. Dir entries sync
// Src/ into Dst/; file and link entries sync Src into Dst.
type Request struct {
	Src, Dst string
	Dir      bool
	Filter   domain.Filter
}

type PlanResult struct {
	Items   []Item
	Warning string
}

const (
	deletingPrefix   = "*deleting   "
	skippingPrefix   = "skipping non-regular file "
	createdDirPrefix = "created directory "
	flagsLen         = 11
	allNewFlags      = "+++++++++"
	linkArrow        = " -> "
	exitVanished     = 24
)

var escapedByte = regexp.MustCompile(`\\#(\d{3})`)

// parseItemize parses --itemize-changes output. Directory noise and rsync's
// informational notices produce no items; skipped non-regular files are returned.
func parseItemize(output string) (items []Item, skipped []string, err error) {
	for _, line := range strings.Split(output, "\n") {
		if line == "" || strings.HasPrefix(line, createdDirPrefix) {
			continue
		}
		if name, ok := strings.CutPrefix(line, skippingPrefix); ok {
			skipped = append(skipped, name)
			continue
		}
		if name, ok := strings.CutPrefix(line, deletingPrefix); ok {
			kind := domain.KindFile
			if strings.HasSuffix(name, "/") {
				kind = domain.KindDir
			}
			items = append(items, Item{unescape(strings.TrimSuffix(name, "/")), domain.Delete, kind})
			continue
		}
		if len(line) <= flagsLen || line[flagsLen] != ' ' {
			return nil, nil, fmt.Errorf("unrecognized rsync output line %q", line)
		}
		flags, name := line[:flagsLen], line[flagsLen+1:]
		var kind domain.NodeKind
		switch flags[1] {
		case 'f':
			kind = domain.KindFile
		case 'L':
			kind = domain.KindLink
			name, _, _ = strings.Cut(name, linkArrow)
		case 'd':
			continue
		default:
			return nil, nil, fmt.Errorf("unsupported entry type in rsync output line %q", line)
		}
		if !strings.ContainsRune("<>ch.", rune(flags[0])) {
			return nil, nil, fmt.Errorf("unrecognized rsync output line %q", line)
		}
		change := domain.Update
		if flags[2:] == allNewFlags {
			change = domain.Create
		}
		items = append(items, Item{unescape(name), change, kind})
	}
	return items, skipped, nil
}

// unescape reverses rsync's \#NNN octal escape; other backslashes are literal.
func unescape(name string) string {
	return escapedByte.ReplaceAllStringFunc(name, func(m string) string {
		n, _ := strconv.ParseUint(m[2:], 8, 8)
		return string([]byte{byte(n)})
	})
}

func baseArgs(r Request) []string {
	args := []string{"--recursive", "--links", "--checksum", "--executability"}
	for _, e := range r.Filter.Excludes {
		args = append(args, "--exclude="+e)
	}
	return args
}

func endpoints(r Request) []string {
	if r.Dir {
		return []string{"--", r.Src + "/", r.Dst + "/"}
	}
	return []string{"--", r.Src, r.Dst}
}

func PlanArgs(r Request) []string {
	args := append(baseArgs(r), "--dry-run", "--itemize-changes", "--delete", "--8-bit-output")
	return append(args, endpoints(r)...)
}

// ApplyArgs builds the copy invocation; filesFrom is used only for Dir requests.
func ApplyArgs(r Request, filesFrom string) []string {
	args := append(baseArgs(r), "--itemize-changes", "--8-bit-output")
	if r.Dir {
		args = append(args, "--files-from="+filesFrom, "--from0")
	}
	return append(args, endpoints(r)...)
}

// planExit maps a Plan exit code: 0 ok, 24 warning, anything else an error.
func planExit(code int) (warning string, err error) {
	switch code {
	case 0:
		return "", nil
	case exitVanished:
		return "some source files vanished during the scan", nil
	}
	return "", fmt.Errorf("rsync exited with code %d", code)
}

func Plan(ctx context.Context, r Request) (PlanResult, error) {
	var out bytes.Buffer
	res, err := execx.Run(ctx, &out, "rsync", PlanArgs(r)...)
	if err != nil {
		return PlanResult{}, err
	}
	warning, err := planExit(res.ExitCode)
	if err != nil {
		return PlanResult{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(res.Stderr))
	}
	items, skipped, err := parseItemize(out.String())
	if err != nil {
		return PlanResult{}, err
	}
	var ws []string
	if warning != "" {
		ws = append(ws, warning)
	}
	if len(skipped) > 0 {
		ws = append(ws, "skipped non-regular files: "+strings.Join(skipped, ", "))
	}
	return PlanResult{Items: items, Warning: strings.Join(ws, "; ")}, nil
}

// itemWriter parses rsync's itemize output line by line as it streams and
// reports each transferred item. Lines that do not parse are ignored: the exit
// code decides success.
type itemWriter struct {
	buf      []byte
	progress func(path string)
}

func (w *itemWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		items, _, err := parseItemize(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
		if err != nil {
			continue
		}
		for _, it := range items {
			w.progress(it.Path)
		}
	}
}

// Apply copies the selected paths (relative to Src) for Dir requests. progress
// (may be nil) is called with each item path as rsync transfers it.
func Apply(ctx context.Context, r Request, paths []string, progress func(path string)) error {
	listPath := ""
	if r.Dir {
		if len(paths) == 0 {
			return nil
		}
		f, err := os.CreateTemp("", "nuydeck-files-from-*")
		if err != nil {
			return err
		}
		listPath = f.Name()
		defer os.Remove(listPath)
		var list strings.Builder
		for _, p := range paths {
			list.WriteString(p + "\x00")
		}
		_, werr := f.WriteString(list.String())
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return werr
		}
	}
	var out io.Writer
	if progress != nil {
		out = &itemWriter{progress: progress}
	}
	res, err := execx.Run(ctx, out, "rsync", ApplyArgs(r, listPath)...)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("rsync exited with code %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}
