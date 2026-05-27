package search

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Suggestion struct {
	Title string `json:"title"`
	Href  string `json:"href"`
}

func SuggestionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeSuggestions(w, nil)
			return
		}

		features, err := featurepkg.FeatureNames(r.Context())
		if err != nil {
			http.Error(w, "Failed to search features", http.StatusInternalServerError)
			return
		}

		matches := matchFeatures(features, query)
		if len(matches) > 8 {
			matches = matches[:8]
		}
		suggestions := make([]Suggestion, 0, len(matches))
		for _, match := range matches {
			suggestions = append(suggestions, Suggestion{Title: match, Href: "/features/" + match})
		}
		writeSuggestions(w, suggestions)
	}
}

func writeSuggestions(w http.ResponseWriter, suggestions []Suggestion) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(suggestions)
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		features, err := featurepkg.FeatureNames(r.Context())
		if err != nil {
			http.Error(w, "Failed to search features", http.StatusInternalServerError)
			return
		}

		matches := matchFeatures(features, query)
		if len(matches) == 1 {
			http.Redirect(w, r, "/features/"+matches[0], http.StatusSeeOther)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Search",
			CurrentPage: components.PageFeatures,
			Content:     searchPage(query, matches),
		})
	}
}

func matchFeatures(features []string, query string) []string {
	q := strings.ToLower(query)
	exact := make([]string, 0, 1)
	matches := make([]string, 0)
	for _, feature := range features {
		name := strings.ToLower(feature)
		if name == q {
			exact = append(exact, feature)
			continue
		}
		if strings.Contains(name, q) {
			matches = append(matches, feature)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	sort.Strings(matches)
	if len(matches) > 25 {
		matches = matches[:25]
	}
	return matches
}

func searchPage(query string, matches []string) g.Node {
	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content landing-page"),
			components.CardCompact(
				h.H1(g.Text("Search")),
				searchForm(query, false, nil),
				g.If(len(matches) == 0, h.P(h.Class("text-muted"), g.Text("No matching features."))),
				g.If(len(matches) > 0, h.Div(h.Class("search-results"),
					g.Group(g.Map(matches, func(feature string) g.Node {
						return h.A(h.Href("/features/"+feature), h.Class("search-result"), g.Text(feature))
					})),
				)),
			),
		),
	)
}

func searchForm(value string, autofocus bool, features []string) g.Node {
	return h.Form(h.Method("get"), h.Action("/search"), h.Class("feature-search-form"), g.Attr("data-feature-search", ""),
		h.Input(
			h.Type("search"),
			h.Name("q"),
			h.Value(value),
			h.Class("feature-search-input"),
			h.Placeholder("Search features…"),
			h.AutoComplete("off"),
			g.Attr("aria-label", "Search features"),
			g.If(autofocus, h.AutoFocus()),
		),
		h.Div(h.Class("feature-search-suggestions"), g.Attr("data-feature-search-suggestions", "")),
	)
}
