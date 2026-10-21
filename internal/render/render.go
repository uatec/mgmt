// Package render executes report templates: a workspace reports/<name>.tmpl
// replaces the embedded core one of the same name.
package render

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// blockedFuncs read the wall clock, the local time zone, the environment,
// the network or a random source, any of which breaks determinism.
var blockedFuncs = []string{
	// clock and local zone
	"now", "ago", "date", "dateInZone", "date_in_zone", "dateModify", "date_modify",
	"mustDateModify", "must_date_modify", "htmlDate", "htmlDateInZone", "toDate",
	"mustToDate", "durationRound",
	// environment and network
	"env", "expandenv", "getHostByName",
	// randomness
	"randAlphaNum", "randAlpha", "randAscii", "randNumeric", "randBytes", "randInt",
	"shuffle", "uuidv4", "genPrivateKey", "genCA", "genCAWithKey", "genSelfSignedCert",
	"genSelfSignedCertWithKey", "genSignedCert", "genSignedCertWithKey",
	"buildCustomCert", "encryptAES", "bcrypt", "htpasswd",
}

// funcs is Sprig's text function map minus blockedFuncs.
func funcs() template.FuncMap {
	m := sprig.TxtFuncMap()
	for _, name := range blockedFuncs {
		delete(m, name)
	}
	return m
}

// Render executes report name over data. The template is
// <wsReportsDir>/<name>.tmpl when that file exists, else reports/<name>.tmpl
// in core. Missing map keys are errors, so templates must guard optional
// data rather than print "<no value>".
func Render(name string, data map[string]any, core fs.FS, wsReportsDir string) (string, error) {
	src, file, err := load(name, core, wsReportsDir)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(file).Funcs(funcs()).Option("missingkey=error").Parse(src)
	if err != nil {
		return "", fmt.Errorf("report %s: %w", file, err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("report %s: %w", file, err)
	}
	return b.String(), nil
}

// load returns the template source and a display name for errors.
func load(name string, core fs.FS, wsReportsDir string) (string, string, error) {
	ws := filepath.Join(wsReportsDir, name+".tmpl")
	b, err := os.ReadFile(ws)
	if err == nil {
		return string(b), filepath.ToSlash(filepath.Join("reports", name+".tmpl")), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", "", err
	}
	corePath := "reports/" + name + ".tmpl"
	b, err = fs.ReadFile(core, corePath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("no report %q: neither %s nor the core template exists", name, ws)
	}
	if err != nil {
		return "", "", err
	}
	return string(b), "core/" + corePath, nil
}
