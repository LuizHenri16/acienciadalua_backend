// Package migrations embeds the golang-migrate SQL files so the API can run
// migrations at startup from a single binary.
package migrations

import "embed"

// FS contains the .up.sql/.down.sql files in this directory.
//
//go:embed *.sql
var FS embed.FS
