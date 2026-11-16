package tomlx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
)

// AppendPackage adds one [[package]] block after the existing text, keeping comments and order.
// The candidate is validated before it replaces the file.
func AppendPackage(path string, entry domain.Package) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	candidate := string(existing)
	if candidate != "" && !strings.HasSuffix(candidate, "\n") {
		candidate += "\n"
	}
	candidate += renderPackage(entry)
	if _, err := parsePackages(filepath.Base(path), []byte(candidate)); err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, []byte(candidate), info.Mode().Perm())
}

func renderPackage(p domain.Package) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n[[package]]\nname = %s\nsource = %s\n", strconv.Quote(p.Name), strconv.Quote(string(p.Source)))
	if len(p.Profiles) > 0 {
		quoted := make([]string, len(p.Profiles))
		for i, profile := range p.Profiles {
			quoted[i] = strconv.Quote(profile)
		}
		fmt.Fprintf(&b, "profiles = [%s]\n", strings.Join(quoted, ", "))
	}
	if p.Group != "" {
		fmt.Fprintf(&b, "group = %s\n", strconv.Quote(p.Group))
	}
	if p.Service != "" {
		fmt.Fprintf(&b, "service = %s\n", strconv.Quote(p.Service))
	}
	return b.String()
}
