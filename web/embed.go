package web

import "embed"

// Assets contains the production frontend built by Task before compiling Go.
//
//go:embed all:dist
var Assets embed.FS
