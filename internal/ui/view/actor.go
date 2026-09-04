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

// ActorName returns just the display name without the workflow link.
// AssignmentCreatorNode renders an assignment creator, including a workflow link when available.
func AssignmentCreatorNode(actor string) g.Node {
	if actor == "" || actor == "unknown" {
		return g.Text("Unknown")
	}
	return ActorNode(actor)
}

// IsWorkflowActor reports whether actor identifies a GitHub Actions run.
func IsWorkflowActor(actor string) bool {
	return ActorWorkflowURL(actor) != ""
}

func ActorName(actor string) g.Node {
	user, rest, ok := strings.Cut(actor, "@")
	if !ok {
		return g.Text(actor)
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[2] != "" {
		return g.Text("@" + user)
	}
	return g.Text(actor)
}

// ActorWorkflowURL returns the GitHub Actions run URL if the actor is a
// GitHub Actions OIDC actor, or empty string otherwise.
func ActorWorkflowURL(actor string) string {
	_, rest, ok := strings.Cut(actor, "@")
	if !ok {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[2] != "" {
		return "https://github.com/" + parts[0] + "/" + parts[1] + "/actions/runs/" + parts[2]
	}
	return ""
}
