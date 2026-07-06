package environment

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HelmValuesFragmentHandler returns an HTML fragment with the computed helm
// values JSON for use in the lazy modal.
func HelmValuesFragmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tenant, err := envpkg.GetTenantByName(ctx, chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Tenant not found", http.StatusNotFound)
			return
		}
		env, err := envpkg.GetByName(ctx, tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Environment not found", http.StatusNotFound)
			return
		}
		feat, err := featureassignment.FeatureForEnvironment(ctx, env.ID, chi.URLParam(r, "feature"))
		if err != nil {
			http.Error(w, "Feature not found", http.StatusNotFound)
			return
		}

		vals, err := featurepkg.HelmValues(ctx, feat, env.ID)
		if err != nil {
			node := helmValuesErrorFragment(err.Error())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = node.Render(w)
			return
		}

		b, err := json.MarshalIndent(vals, "", "  ")
		if err != nil {
			http.Error(w, "Failed to encode values", http.StatusInternalServerError)
			return
		}

		node := helmValuesFragment(string(b))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = node.Render(w)
	}
}

func helmValuesFragment(valuesJSON string) g.Node {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(valuesJSON), "", "  "); err == nil {
		valuesJSON = pretty.String()
	}
	return h.Div(
		h.Class("modal-body"),
		h.H3(g.Text("Computed Helm Values")),
		h.Div(
			h.Class("code-block-wrap"),
			h.Button(h.Type("button"), h.Class("copy-btn"), g.Attr("data-copy-target", "helm-values-modal"), g.Text("Copy")),
			h.Pre(h.Class("code-block"), h.ID("helm-values-modal"), g.Text(valuesJSON)),
		),
	)
}

func helmValuesErrorFragment(errMsg string) g.Node {
	return h.Div(
		h.Class("modal-body"),
		h.H3(g.Text("Computed Helm Values")),
		h.P(g.Text("Failed to render helm values:")),
		h.Pre(h.Class("code-block"), g.Text(errMsg)),
	)
}
