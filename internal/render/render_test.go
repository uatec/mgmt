package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

var core = fstest.MapFS{
	"reports/prep.tmpl": {Data: []byte("core {{.person}}\n")},
}

func TestRenderCore(t *testing.T) {
	got, err := Render("prep", map[string]any{"person": "sam"}, core, filepath.Join(t.TempDir(), "reports"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "core sam\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderOverride(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "prep.tmpl"), []byte("workspace {{.person | upper}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Render("prep", map[string]any{"person": "sam"}, core, ws)
	if err != nil {
		t.Fatal(err)
	}
	if got != "workspace SAM\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderNoClock(t *testing.T) {
	for _, fn := range []string{"now", "ago", "date", "env", "expandenv", "getHostByName", "randAlpha", "uuidv4"} {
		ws := t.TempDir()
		src := "{{ " + fn + " }}"
		if err := os.WriteFile(filepath.Join(ws, "prep.tmpl"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Render("prep", map[string]any{}, core, ws)
		if err == nil || !strings.Contains(err.Error(), `function "`+fn+`" not defined`) {
			t.Errorf("%s: err = %v, want parse error", fn, err)
		}
	}
}

func TestRenderMissingKeyErrors(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "prep.tmpl"), []byte("{{.nope}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Render("prep", map[string]any{}, core, ws); err == nil {
		t.Error("missing key rendered without error")
	}
}

func TestRenderUnknownReport(t *testing.T) {
	_, err := Render("bogus", map[string]any{}, core, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `no report "bogus"`) {
		t.Errorf("err = %v", err)
	}
}
