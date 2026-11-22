package cfgsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"

	"github.com/Andr3xDev/nuydeck/tui/internal/editor"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const editFileMode = fs.FileMode(0o600)

// ErrNotEditable means the destination is missing, binary, too large or a link.
var ErrNotEditable = errors.New("destination is not an editable text file")

// TempFS is the private scratch space the editor works in.
type TempFS interface {
	MkdirPrivate() (string, error)
	WriteFile(path string, data []byte, mode fs.FileMode) error
	ReadFile(path string) ([]byte, error)
	RemoveAll(path string) error
}

// EditPorts are the outside world ComposeEdit needs.
type EditPorts struct {
	Getenv     func(string) string
	HasCommand func(string) bool
	Terminal   plan.Terminal
	Temp       TempFS
	ReadSide   func(path string) (Side, error)
}

// EditRequest names the two sides; Base is the display name used for the temp file names.
type EditRequest struct{ Destination, Source, Base string }

// EditResult carries the edited content; Mode is the destination's.
type EditResult struct {
	Changed bool
	Content []byte
	Mode    fs.FileMode
	Notices []string
}

// ComposeEdit lets the user edit the destination with the source beside it. Neither side is written.
func ComposeEdit(ctx context.Context, ports EditPorts, req EditRequest) (res EditResult, err error) {
	dst, err := ports.ReadSide(req.Destination)
	if err != nil {
		return EditResult{}, err
	}
	if !dst.Exists || dst.Kind != plan.KindFile || dst.Binary || dst.TooLarge {
		return EditResult{}, ErrNotEditable
	}
	src, err := ports.ReadSide(req.Source)
	if err != nil {
		return EditResult{}, err
	}
	argv, err := editor.Resolve(ports.Getenv, ports.HasCommand)
	if err != nil {
		return EditResult{}, err
	}

	dir, err := ports.Temp.MkdirPrivate()
	if err != nil {
		return EditResult{}, err
	}
	defer func() {
		if rmErr := ports.Temp.RemoveAll(dir); rmErr != nil {
			res.Notices = append(res.Notices, fmt.Sprintf("could not remove temporary files in %s: %v", dir, rmErr))
		}
	}()

	destPath := filepath.Join(dir, "destination."+req.Base)
	srcPath := filepath.Join(dir, "source."+req.Base)
	resultPath := filepath.Join(dir, "result."+req.Base)
	for path, content := range map[string][]byte{destPath: dst.Content, srcPath: src.Content, resultPath: dst.Content} {
		if err := ports.Temp.WriteFile(path, content, editFileMode); err != nil {
			return EditResult{}, err
		}
	}

	cmd := exec.Command(argv[0], append(argv[1:], resultPath, destPath, srcPath)...)
	if err := ports.Terminal.Run(ctx, cmd); err != nil {
		return EditResult{}, fmt.Errorf("editor: %w", err)
	}
	edited, err := ports.Temp.ReadFile(resultPath)
	if err != nil {
		return EditResult{}, err
	}
	return EditResult{Changed: !bytes.Equal(edited, dst.Content), Content: edited, Mode: dst.Mode}, nil
}
