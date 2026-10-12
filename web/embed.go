// Package web embeds the overlays, themes and dashboard served by the local server.
package web

import "embed"

// FS holds the overlays, themes and dashboard directories.
//
//go:embed overlays themes dashboard
var FS embed.FS
