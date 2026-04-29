# UI Layer

Server-rendered HTML using [gomponents](https://www.gomponents.com/) (pure Go HTML builder). No JavaScript framework; minimal vanilla JS for theme toggle only. Routing via [chi](https://github.com/go-chi/chi).

## Directory Structure

```
ui/
  server/        → HTTP server, chi router, static asset handlers, logging middleware
  layout/        → Page shell: HTML5 boilerplate, head tags, body wrapper
  components/    → Shared UI primitives: SiteHeader, nav, Page type constants
  pages/         → Page handlers organized by domain:
    tenants/     → Tenant list (root page)
    tenant/      → Single tenant with environment list
    environment/ → Environment detail, feature tabs (overview/logs/helm/rollouts/audit/playground)
    features/    → Feature list, feature detail tabs (overview/status/deployments/rollouts)
    rollouts/    → Rollout list/detail + deployment list/detail/logs (deployment.go is separate from rollouts.go)
    labels/      → Label management
  breadcrumb/    → Breadcrumb navigation builder (supports dropdown switcher via Alternatives)
  chart/         → Chart/visualization components
  view/          → View helper (renderPage function type)
  site/          → Static assets: site.css, site.js (embedded via embed.go)
  embed.go       → go:embed directives for CSS/JS
```

## Conventions

### Handler Pattern

Every page follows the same pattern — a handler function that returns `http.HandlerFunc`, wired in `server/routes.go`:

```go
// In pages/features/features.go
func ListHandler(renderPage view.RenderPageFunc, repo database.Repo) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Extract URL params via chi.URLParam(r, "feature")
        // 2. Query data via context-loaded packages: feature.X(ctx, ...), deployment.X(ctx, ...)
        // 3. Build gomponents tree
        // 4. Call renderPage(w, r, layout.Props{...})
    }
}

// In server/routes.go
r.Get("/features", features.ListHandler(s.renderPage, s.repo))
```

### gomponents Usage

HTML is built with Go functions, not templates. Import conventions:

```go
import (
    g "maragu.dev/gomponents"           // Core: Text, Raw, Attr, Group, If, Map
    c "maragu.dev/gomponents/components" // HTML5 helper
    h "maragu.dev/gomponents/html"       // HTML elements: Div, Span, A, Table, etc.
)
```

Key patterns:
- `g.If(condition, node)` — conditional rendering
- `g.Map(slice, func)` — list rendering
- `h.Class("...")` — CSS classes
- Elements are just function calls: `h.Div(h.Class("container"), h.Span(g.Text("hello")))`

### Page Navigation

`components.Page` is a typed string constant used for nav highlighting:

```go
type Page string
const (
    PageTenants     Page = "tenants"
    PageFeatures    Page = "features"
    PageDeployments Page = "deployments"
    PageRollouts    Page = "rollouts"
    PageLabels      Page = "labels"
)
```

Set `CurrentPage` in `layout.Props` to highlight the correct nav item. Every handler must set this.

### Routing

All routes defined in `server/routes.go`. URL structure:
- `/` → tenants list
- `/tenants/{tenant}` → tenant detail
- `/tenants/{tenant}/envs/{env}` → environment detail
- `/tenants/{tenant}/envs/{env}/{feature}` → feature in environment (tabbed)
- `/features` → feature list
- `/features/{feature}` → feature detail (tabbed: overview/status/deployments/rollouts)
- `/rollouts` → rollout list
- `/deployments` → deployment list
- `/deployments/{id}` → deployment detail
- `/labels` → labels

Tabbed pages use a shared handler with a tab parameter: `TabHandler(renderPage, repo, "status")`.

POST routes use form submissions for destructive actions (delete, bulk-delete). No client-side fetch calls.

### Breadcrumbs

Breadcrumbs support a dropdown "switcher" for navigation between siblings (e.g., switching tenants or environments). Built with the `Alternatives` field on `breadcrumb.Crumb`:

```go
breadcrumb.TenantWithSwitcher(name, allTenants)        // Dropdown to switch tenant
breadcrumb.EnvironmentWithSwitcher(tenant, env, envs)   // Dropdown to switch environment
```

When `Alternatives` is non-empty, `components.Breadcrumbs` renders a dropdown instead of a plain link.

### Static Assets

CSS and JS are embedded at compile time via `embed.go`. Served at `/site.css` and `/site.js` by `server/assets.go`. No build step for frontend assets.

### Data Access

UI handlers access data through the context loader pattern (see root AGENTS.md). They call package-level functions like `feature.AllFeatures(ctx)`, `deployment.ListDeployments(ctx)`, etc. The `database.Repo` is passed for operations that still use the shared repo interface.

## Anti-Patterns

- No JavaScript frameworks. Vanilla JS only, and only for theme toggle.
- No client-side routing. All navigation is server-side `<a>` tags.
- No template files. All HTML is Go code via gomponents.
- Do not add new JS without discussion.
- Time formatting uses `mustLoadLocation("Europe/Oslo")` — defined per-page, not centralized.
