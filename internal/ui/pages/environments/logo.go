package environments

import (
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	ui "github.com/nais/fasit/internal/ui"
)

func ServeLogoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantName := filepath.Base(chi.URLParam(r, "tenant"))
		data, err := fs.ReadFile(ui.SiteFS, "site/logos/"+tenantName+".svg")
		if err != nil {
			data, err = fs.ReadFile(ui.SiteFS, "site/logos/default.svg")
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}

		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(data) // #nosec G104,G705 -- embedded static SVG, not user-supplied
	}
}
