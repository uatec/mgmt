package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	mgmt "github.com/uatec/mgmt"
)

// runInit writes the embedded scaffold into root. It creates new files only:
// if any target exists it writes nothing and exits 2.
func runInit(args []string, root string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		return usageError(stderr, "init takes no arguments")
	}
	var paths []string
	err := fs.WalkDir(mgmt.Scaffold, "scaffold", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p[len("scaffold/"):])
		}
		return err
	})
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if _, err := os.Lstat(filepath.Join(root, rel)); err == nil {
			fmt.Fprintf(stderr, "mgmt: refusing to overwrite %s\n", rel)
			return 2
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "mgmt: %v\n", err)
			return 2
		}
	}
	for _, rel := range paths {
		data, err := mgmt.Scaffold.ReadFile("scaffold/" + rel)
		if err == nil {
			dst := filepath.Join(root, rel)
			if err = os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
				err = os.WriteFile(dst, data, 0o644)
			}
		}
		if err != nil {
			fmt.Fprintf(stderr, "mgmt: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, rel)
	}
	return 0
}
