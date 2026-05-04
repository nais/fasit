# UI Layer

Server-rendered HTML using [gomponents](https://www.gomponents.com/) (pure Go HTML builder). No JavaScript framework; minimal vanilla JS for theme toggle only. Routing via [chi](https://github.com/go-chi/chi).

## Directory Structure

```
ui/
  server/        → HTTP server, chi router, static asset handlers, logging middleware
  layout/        → Page shell: HTML5 boilerplate, head tags, body wrapper
  components/    → SiteHeader, nav, Page type constants
  pages/         → Page handlers organized by domain:
    deployments/ → Deployment list/detail/logs
    environment/ → Environment detail, feature tabs (overview/logs/helm/rollouts/audit/playground)
    features/    → Feature list, feature detail tabs (overview/status/deployments/rollouts)
    rollouts/    → Rollout list/detail
  breadcrumb/    → Supports dropdown switcher via Alternatives
  view/          → renderPage function type
  site/          → Static assets: site.css, site.js (embedded via embed.go)
```

## Conventions

- **Handler pattern**: Every page returns `http.HandlerFunc`, wired in `server/routes.go`. Extract URL params via `chi.URLParam`, query data via context-loaded packages, build gomponents tree, call `renderPage(w, layout.Props{...})`.
- **gomponents**: Import as `g "maragu.dev/gomponents"`, `h "maragu.dev/gomponents/html"`. Key: `g.If()`, `g.Map()`, `h.Class()`.
- **Nav highlighting**: Set `CurrentPage` (typed `components.Page` constant) in `layout.Props`. Every handler must set this.
- **Tabbed pages**: Shared handler with tab parameter: `TabHandler(renderPage, repo, "status")`.
- **Breadcrumb switcher**: `breadcrumb.Crumb` has `Alternatives []Crumb` — renders dropdown. Use `TenantWithSwitcher` / `EnvironmentWithSwitcher`.
- **Destructive actions**: POST forms with confirm JS. No client-side fetch calls.
- **Time formatting**: Pages use `mustLoadLocation("Europe/Oslo")` — defined per-page, not centralized.

## Anti-Patterns

- No JavaScript frameworks — vanilla JS only, theme toggle only
- No client-side routing — server-side `<a>` tags only
- No template files — all HTML is Go code via gomponents
- No new JS without discussion

## Rollouts vs Deployments (UI)

- **Separate nav items**: `PageRollouts` and `PageDeployments` in `components/header.go`
- **Separate breadcrumbs**: `breadcrumb.Rollouts()` and `breadcrumb.Deployments()` — never cross-reference
- **Separate packages**: `pages/rollouts/` and `pages/deployments/` are independent. Some helpers (`rolloutStatus`, `formatTime`, `versionCell`, `metaRow`) are duplicated across both packages by design — keep them aligned but don't extract to a shared package.
- **Feature detail page**: Has separate "Rollouts" and "Deployments" tabs with independent data types (`RolloutItem` vs `DeploymentItem`)
- **Environment feature page**: Conditionally shows either "Deployments" or "Rollouts" tab based on `HasDeployments` — never both
