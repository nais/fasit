package ui

import "embed"

const BasePath = "/ui"

//go:embed site/*.css site/*.js
var SiteFS embed.FS
