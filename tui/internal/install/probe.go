package install

import "context"

// SystemProbe holds the read-only machine queries of the install and the package screens; always the real implementation.
type SystemProbe interface {
	DetectGPUs(ctx context.Context) ([]PCIDevice, error)
	InstalledPackages(ctx context.Context) (map[string]bool, error)
	HasCommand(name string) bool
	LoginShell(ctx context.Context) (string, error)
	// ReadSystemFile reads an allow-listed system file; a missing file returns an error wrapping fs.ErrNotExist.
	ReadSystemFile(path string) ([]byte, error)
	IsRoot() bool
	// InGroup reports whether the user belongs to the named group.
	InGroup(ctx context.Context, group string) (bool, error)
}
