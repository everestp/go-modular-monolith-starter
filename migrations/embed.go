// Package migrations embeds the versioned SQL migrations for the application.
package migrations

import "embed"

// Files contains all up and down migration scripts.
//
//go:embed *.sql
var Files embed.FS
