package components

import (
	"io/fs"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/nais/fasit/internal/ui"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HasTenantLogo reports whether an embedded SVG logo exists for the tenant.
func HasTenantLogo(tenantName string) bool {
	safe := filepath.Base(tenantName)
	_, err := fs.Stat(ui.SiteFS, "site/logos/"+safe+".svg")
	return err == nil
}

// TenantAvatar renders a tenant logo (if hasLogo) or an initial-letter fallback.
// size is the CSS dimension (e.g. "24px", "48px").
func TenantAvatar(tenantName string, hasLogo bool, size string) g.Node {
	if hasLogo {
		return h.Img(
			h.Src("/tenants/"+tenantName+"/logo"),
			h.Alt(tenantName),
			h.Class("tenant-avatar"),
			h.Style("width:"+size+";height:"+size+";"),
		)
	}
	initial := tenantInitial(tenantName)
	color := initialColor(tenantName)
	return h.Span(
		h.Class("tenant-avatar tenant-avatar-fallback"),
		h.Style("width:"+size+";height:"+size+";background:"+color+";font-size:calc("+size+" * 0.5);"),
		g.Text(initial),
	)
}

func tenantInitial(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "?"
}

func initialColor(name string) string {
	colors := []string{
		"#4e79a7", "#f28e2b", "#e15759", "#76b7b2",
		"#59a14f", "#edc948", "#b07aa1", "#ff9da7",
	}
	var hash uint32
	for i := 0; i < len(name); i++ {
		hash = hash*31 + uint32(name[i])
	}
	return colors[int(hash)%len(colors)]
}
