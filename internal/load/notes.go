package load

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// NoteFile is a notes file read from disk.
type NoteFile struct {
	// Path is slash-separated: relative to the workspace root for files under
	// it, otherwise the absolute, cleaned path (so file:line stays clickable).
	Path   string
	Source []byte
}

// LoadNotes reads every file matching the doublestar glob, sorted by Path.
// The glob is relative to root, absolute, or starts with "~/" (the user's
// home directory). A glob that matches nothing yields an empty slice and no
// error; a malformed glob returns doublestar.ErrBadPattern.
func LoadNotes(root, glob string) ([]NoteFile, error) {
	if glob == "~" || strings.HasPrefix(glob, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return nil, fmt.Errorf("spec.notes %q: cannot resolve the home directory (is $HOME set?)", glob)
		}
		glob = filepath.ToSlash(home) + glob[1:]
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// os.DirFS rejects "..", so resolve such globs against the root first.
	for _, seg := range strings.Split(filepath.ToSlash(glob), "/") {
		if seg == ".." && !filepath.IsAbs(glob) {
			glob = filepath.ToSlash(filepath.Join(absRoot, filepath.FromSlash(glob)))
			break
		}
	}
	base, pattern, rooted := root, glob, false
	if strings.HasPrefix(glob, "/") || filepath.IsAbs(glob) {
		base, pattern = doublestar.SplitPattern(filepath.ToSlash(glob))
		rooted = true
	}
	matches, err := doublestar.Glob(os.DirFS(base), pattern)
	if err != nil {
		return nil, err
	}
	notes := []NoteFile{}
	for _, m := range matches {
		full := filepath.Join(base, filepath.FromSlash(m))
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		p := m
		if rooted {
			p = noteDisplayPath(absRoot, full)
		}
		notes = append(notes, NoteFile{Path: p, Source: b})
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })
	return notes, nil
}

// noteDisplayPath is full relative to absRoot when it lies under it,
// otherwise the cleaned absolute path, always slash-separated.
func noteDisplayPath(absRoot, full string) string {
	if rel, err := filepath.Rel(absRoot, full); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path.Clean(filepath.ToSlash(full))
}
