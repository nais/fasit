package ui

import "embed"

//go:embed site/*.css site/*.js site/favicon.ico
var SiteFS embed.FS
