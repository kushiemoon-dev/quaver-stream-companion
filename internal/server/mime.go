package server

import "mime"

var fixedTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".ttf":   "font/ttf",
	".woff2": "font/woff2",
	".png":   "image/png",
}

func init() { registerMimeTypes() }

// registerMimeTypes overrides what the OS declares, which varies per machine.
func registerMimeTypes() {
	for ext, typ := range fixedTypes {
		_ = mime.AddExtensionType(ext, typ)
	}
}
