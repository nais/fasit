package ui

import "embed"

//go:embed site/*.css site/*.js
var SiteFS embed.FS
