package features

import (
	"math/rand/v2"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type DetailPage struct {
	Breadcrumbs    []breadcrumb.Crumb
	Features       []view.FeatureNav
	CurrentFeature *model.Feature
	DeploymentEnvs []DeploymentEnvStatus
	Prefs          ViewPrefs
}

func ListHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.FeatureNames(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{Title: "Features", CurrentPage: components.PageFeatures, Content: listPage(toFeatureNavs(features))})
	}
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r, repo)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.Prefs = parseViewPrefs(r)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav) g.Node {
	return h.Div(h.Class("container"), components.FeaturesSidebar(features, ""), h.Main(h.Class("main-content"), components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Features()}), h.Div(h.Class("card"), h.Div(h.Class("card-body"), jokeOfTheMoment()))))
}

func jokeOfTheMoment() g.Node {
	jokes := []struct{ Q, A string }{
		{"Why did the feature flag break up with the deployment?", "It just couldn’t commit."},
		{"How many SREs does it take to change a light bulb?", "None — it’s a hardware problem, page the on-call."},
		{"Why don’t Helm charts ever get invited to parties?", "They always bring too many values."},
		{"What did the rollout say to the deployment?", "“Stop overriding me, you’re not my real parent.”"},
		{"Why did the YAML file go to therapy?", "Indentation issues."},
		{"What’s a Kubernetes cluster’s favorite music?", "Heavy metal — because everything’s controlled by operators."},
		{"Why did naisd cross the road?", "To reconcile the other side."},
		{"Why was the OCI registry always calm?", "It had great pull."},
		{"How does a feature deploy itself?", "With a little Helm."},
		{"What did the PENDING status say to the DEPLOYED status?", "“Must be nice.”"},
	}
	j := jokes[rand.IntN(len(jokes))] // #nosec G404 -- joke picker, no security relevance
	return h.Div(h.Class("joke"),
		h.P(h.Class("joke-q"), g.Text(j.Q)),
		h.P(h.Class("joke-a"), g.Text(j.A)),
		h.P(h.Class("text-muted joke-hint"), g.Text("Pick a feature from the sidebar to get back to work.")),
	)
}

func detailPage(data *DetailPage) g.Node {
	content := deploymentDetailContent(data)
	return h.Div(h.Class("container"),
		components.FeaturesSidebar(data.Features, data.CurrentFeature.Name),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(data.Breadcrumbs),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), content)),
		),
	)
}

func loadFeatureData(r *http.Request, repo database.Repo) (*DetailPage, error) {
	featureName := chi.URLParam(r, "feature")
	features, err := featurepkg.FeatureNames(r.Context())
	if err != nil {
		return nil, err
	}
	feature, err := featurepkg.FeatureByName(r.Context(), featureName)
	if err != nil {
		return nil, err
	}
	featureCrumb := breadcrumb.Feature(featureName)
	featureCrumb.Subtitle = feature.Description
	featureCrumb.SourceURL = feature.Source
	data := &DetailPage{
		Breadcrumbs:    []breadcrumb.Crumb{breadcrumb.Features(), featureCrumb},
		Features:       toFeatureNavs(features),
		CurrentFeature: feature,
	}
	loadDeploymentData(r.Context(), repo, feature, data)
	return data, nil
}

func toFeatureNavs(names []string) []view.FeatureNav {
	ret := make([]view.FeatureNav, 0, len(names))
	for _, name := range names {
		ret = append(ret, view.FeatureNav{
			Name: name,
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret
}

func featureTargetsKind(kinds []model.EnvironmentKind, envKind model.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}

func lastDeployedCell(t time.Time, extraTitle string) g.Node {
	if t.IsZero() {
		if extraTitle != "" {
			return h.Td(h.Title(extraTitle), h.Span(h.Class("text-muted"), g.Text("never")))
		}
		return h.Td(h.Span(h.Class("text-muted"), g.Text("never")))
	}
	title := view.FormatTime(t)
	if extraTitle != "" {
		title = extraTitle
	}
	return h.Td(h.Title(title), g.Text(view.RelativeTime(t)))
}

func renderStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	case "DISABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("○")), g.Text(" DISABLED")})
	case "CREATED":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" CREATED")})
	case "UNKNOWN":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("?")), g.Text(" UNKNOWN")})
	case "ENABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" ENABLED")})
	case "OVERRIDDEN":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("⊘")), g.Text(" OVERRIDDEN")})
	default:
		return g.Text(status)
	}
}
