// Package web embeds the minimal UI (index.html + app.js) so the connector binary
// is self-contained: no separate static service, no CORS to configure.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html app.js
var files embed.FS

// FS returns the embedded UI filesystem rooted at /.
func FS() fs.FS { return files }
