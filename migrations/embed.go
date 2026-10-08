// Package migrations holds the SQL goose applies on startup.
// The files are embedded so the distroless image does not need a migrations
// directory or a second binary.
package migrations

import "embed"

// FS is the set of *.sql files in this directory.
//
//go:embed *.sql
var FS embed.FS
