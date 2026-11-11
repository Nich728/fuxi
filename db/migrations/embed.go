// Package migrations embeds the goose SQL migrations into the fuxi binary,
// so `fuxi migrate up` needs no files on disk at release time.
//
// Never edit a migration after it is merged; add a new one instead.
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
