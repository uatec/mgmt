// Package mgmt embeds the core schema, policies, report templates and scaffold files.
package mgmt

import "embed"

// CoreSchema holds the core CUE schema.
//
//go:embed schema
var CoreSchema embed.FS

// CorePolicy holds the core Rego policies.
//
//go:embed policy/core
var CorePolicy embed.FS

// CoreReports holds the report templates.
//
//go:embed reports
var CoreReports embed.FS

// Scaffold holds the files written by `mgmt init`.
//
//go:embed scaffold
var Scaffold embed.FS
