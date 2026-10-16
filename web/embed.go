// Package web embeds the dashboard UI.
package web

import "embed"

//go:embed index.html style.css app.js
var FS embed.FS
