// Package load reads resources and notes from a workspace.
package load

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/uatec/mgmt/internal/facts"
)

// Resource is one YAML document with its location.
type Resource struct {
	APIVersion string
	Kind       string
	Name       string
	Object     map[string]any
	File       string // relative to the workspace root, slash-separated
	Line       int    // 1-based line of the document's first line
}

var timestampRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T00:00:00Z$`)

// LoadResources reads resources/**/*.{yaml,yml} under root, sorted by path then
// document order. Unparseable or incomplete documents become schema-invalid
// findings; the remaining documents still load.
func LoadResources(root string) ([]Resource, []facts.Finding) {
	var paths []string
	base := filepath.Join(root, "resources")
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(p)); ext == ".yaml" || ext == ".yml" {
			rel, rerr := filepath.Rel(root, p)
			if rerr == nil {
				paths = append(paths, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(paths)

	var res []Resource
	var finds []facts.Finding
	for _, rel := range paths {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			finds = append(finds, schemaInvalid(rel, 1, err.Error()))
			continue
		}
		r, f := parseDocs(rel, src)
		res = append(res, r...)
		finds = append(finds, f...)
	}
	return res, finds
}

type doc struct {
	line int
	text string
}

// splitDocs strips a BOM and CRs, then splits on lines that are exactly "---".
func splitDocs(src []byte) []doc {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var docs []doc
	var cur []string
	start := 1
	flush := func() {
		docs = append(docs, doc{line: start, text: strings.Join(cur, "\n")})
	}
	for i, l := range strings.Split(text, "\n") {
		if l == "---" {
			flush()
			cur, start = nil, i+2
			continue
		}
		cur = append(cur, l)
	}
	flush()
	return docs
}

func parseDocs(file string, src []byte) ([]Resource, []facts.Finding) {
	var res []Resource
	var finds []facts.Finding
	for _, d := range splitDocs(src) {
		if strings.TrimSpace(d.text) == "" {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(d.text), &obj); err != nil {
			finds = append(finds, schemaInvalid(file, d.line, "invalid YAML: "+oneLine(err.Error())))
			continue
		}
		if obj == nil {
			continue // comments only
		}
		obj = normalise(obj).(map[string]any)
		kind, _ := obj["kind"].(string)
		meta, _ := obj["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		if kind == "" || name == "" {
			finds = append(finds, schemaInvalid(file, d.line, "resource needs kind and metadata.name"))
			continue
		}
		apiVersion, _ := obj["apiVersion"].(string)
		res = append(res, Resource{APIVersion: apiVersion, Kind: kind, Name: name, Object: obj, File: file, Line: d.line})
	}
	return res, finds
}

// normalise turns timestamp scalars back into YYYY-MM-DD strings.
func normalise(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = normalise(e)
		}
	case []any:
		for i, e := range t {
			t[i] = normalise(e)
		}
	case time.Time:
		return t.Format("2006-01-02")
	case string:
		if timestampRE.MatchString(t) {
			return t[:10]
		}
	}
	return v
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func schemaInvalid(file string, line int, msg string) facts.Finding {
	return facts.Finding{Severity: "error", ID: "schema-invalid", File: file, Line: line, Message: msg}
}
