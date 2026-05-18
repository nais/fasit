package tenants

import (
	"net/http"
	"sort"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type envEntry struct {
	Tenant      string
	Environment string
	Labels      map[string]string
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := environment.GetTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load environments", http.StatusInternalServerError)
			return
		}

		var envs []envEntry
		allKeys := map[string]bool{}

		for _, tenant := range tenants {
			environments, err := repo.EnvironmentsGet(r.Context(), tenant.ID)
			if err != nil {
				continue
			}
			for _, env := range environments {
				labels, err := repo.EnvironmentGetLabels(r.Context(), env.ID)
				if err != nil {
					continue
				}
				labelMap := make(map[string]string, len(labels))
				for _, l := range labels {
					labelMap[l.Key] = l.Value
					allKeys[l.Key] = true
				}
				envs = append(envs, envEntry{
					Tenant:      tenant.Name,
					Environment: env.Name,
					Labels:      labelMap,
				})
			}
		}

		groupBy := r.URL.Query().Get("group")
		if groupBy == "" {
			groupBy = "tenant"
		}

		keys := make([]string, 0, len(allKeys))
		for k := range allKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		renderPage(w, r, layout.Props{
			Title:       "Environments",
			CurrentPage: components.PageEnvironments,
			Content:     page(envs, keys, groupBy),
		})
	}
}

func page(envs []envEntry, labelKeys []string, groupBy string) g.Node {
	groups := groupEnvs(envs, groupBy)

	facetLinks := make([]g.Node, 0, len(labelKeys))
	for _, key := range labelKeys {
		class := "facet-link"
		if key == groupBy {
			class += " active"
		}
		facetLinks = append(facetLinks, h.A(
			h.Href("/environments?group="+key),
			h.Class(class),
			g.Text(key),
		))
	}

	return h.Div(h.Class("environments-page"),
		h.Div(h.Class("environments-facets"),
			h.Span(h.Class("facets-label"), g.Text("Group by: ")),
			g.Group(facetLinks),
		),
		h.Div(h.Class("dashboard"), g.Group(
			groupCards(groups, groupBy),
		)),
	)
}

type envGroup struct {
	Value string
	Envs  []envEntry
}

func groupEnvs(envs []envEntry, key string) []envGroup {
	groupMap := map[string][]envEntry{}
	var order []string

	for _, env := range envs {
		val := env.Labels[key]
		if val == "" {
			val = "(none)"
		}
		if _, ok := groupMap[val]; !ok {
			order = append(order, val)
		}
		groupMap[val] = append(groupMap[val], env)
	}

	sort.Strings(order)

	groups := make([]envGroup, 0, len(order))
	for _, val := range order {
		groups = append(groups, envGroup{Value: val, Envs: groupMap[val]})
	}
	return groups
}

func groupCards(groups []envGroup, groupBy string) []g.Node {
	cards := make([]g.Node, 0, len(groups))
	for _, group := range groups {
		// Sort envs within each group
		sort.Slice(group.Envs, func(i, j int) bool {
			if group.Envs[i].Tenant != group.Envs[j].Tenant {
				return group.Envs[i].Tenant < group.Envs[j].Tenant
			}
			return group.Envs[i].Environment < group.Envs[j].Environment
		})

		items := make([]g.Node, 0, len(group.Envs))
		for _, env := range group.Envs {
			// Show tenant prefix when not grouping by tenant
			label := env.Environment
			if groupBy != "tenant" {
				label = env.Tenant + " / " + env.Environment
			}
			items = append(items, h.Li(
				h.A(h.Href("/tenants/"+env.Tenant+"/envs/"+env.Environment),
					g.Text(label),
				),
			))
		}

		var heading g.Node
		if groupBy == "tenant" {
			heading = h.A(h.Href("/tenants/"+group.Value), g.Text(group.Value))
		} else {
			heading = g.Text(group.Value)
		}

		cards = append(cards, h.Article(h.Class("dash-card"),
			h.H3(heading),
			h.Ul(g.Group(items)),
		))
	}
	return cards
}
