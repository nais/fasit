package view

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ActorNode renders an audit actor string. GitHub Actions OIDC actors
// (format "user@org/repo/runId") are rendered as "@user via workflow" where
// "workflow" links to the GitHub Actions run. Plain emails render as-is.
func ActorNode(actor string) g.Node {
	user, rest, ok := strings.Cut(actor, "@")
	if !ok {
		return g.Text(actor)
	}

	// Check if rest matches org/repo/runId pattern (3 segments)
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[2] != "" {
		org, repo, runID := parts[0], parts[1], parts[2]
		href := "https://github.com/" + org + "/" + repo + "/actions/runs/" + runID
		return g.Group([]g.Node{
			g.Text("@" + user + " via "),
			h.A(h.Href(href), g.Attr("target", "_blank"), g.Attr("rel", "noopener noreferrer"), g.Text("workflow")),
		})
	}

	return g.Text(actor)
}
