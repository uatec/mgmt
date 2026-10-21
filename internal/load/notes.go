package load

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
)

// NoteFile is a notes file read from the workspace.
type NoteFile struct {
	Path   string // relative to the workspace root, slash-separated
	Source []byte
}

// LoadNotes reads every file under root matching the doublestar glob, sorted by Path.
// A glob that matches nothing yields an empty slice and no error.
func LoadNotes(root, glob string) ([]NoteFile, error) {
	matches, err := doublestar.Glob(os.DirFS(root), glob)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	notes := []NoteFile{}
	for _, m := range matches {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(m)))
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(m)))
		if err != nil {
			return nil, err
		}
		notes = append(notes, NoteFile{Path: m, Source: b})
	}
	return notes, nil
}
