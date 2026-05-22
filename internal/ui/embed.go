package ui

import "embed"

//go:embed site/*.css site/*.js site/favicon.ico site/logos/*.svg
var SiteFS embed.FS
