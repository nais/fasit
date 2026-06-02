package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// RedeployPopover renders a confirmation popover for triggering a redeploy.
// Returns nil if enabled is false. If redirectURL is non-empty, the form
// includes a hidden field so the handler redirects back to that URL.
func RedeployPopover(popoverID, action, featureName, envName string, enabled bool, redirectURL string) g.Node {
	if !enabled {
		return nil
	}
	return h.Div(g.Attr("popover", ""), h.ID(popoverID),
		h.H3(g.Text("Confirm redeploy")),
		h.Form(h.Method("POST"), h.Action(action),
			redirectField(redirectURL),
			h.P(g.Textf("Force a fresh deploy of %s in %s?", featureName, envName)),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Trigger redeploy")),
				h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

// ReconcilePopover renders a confirmation popover for toggling reconcile.
// If redirectURL is non-empty, the form includes a hidden field so the
// handler redirects back to that URL.
func ReconcilePopover(popoverID, action, featureName, envName string, enabled bool, redirectURL string) g.Node {
	if enabled {
		reasonID := popoverID + "-reason"
		return h.Div(g.Attr("popover", ""), h.ID(popoverID),
			h.H3(g.Text("Disable reconcile")),
			h.Form(h.Method("POST"), h.Action(action),
				h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("false")),
				redirectField(redirectURL),
				h.P(g.Textf("Disable reconcile for %s in %s? Reconciliation will stop until re-enabled.", featureName, envName)),
				h.Label(h.For(reasonID), g.Text("Reason for disabling reconcile")),
				h.Textarea(h.ID(reasonID), h.Name("reason"), g.Attr("maxlength", "1000"), g.Attr("required", ""), h.Rows("3")),
				h.Div(h.Class("popover-actions"),
					h.Button(h.Type("submit"), g.Text("Disable reconcile")),
					h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
				),
			),
		)
	}

	return h.Div(g.Attr("popover", ""), h.ID(popoverID),
		h.H3(g.Text("Enable reconcile")),
		h.Form(h.Method("POST"), h.Action(action),
			h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("true")),
			redirectField(redirectURL),
			h.P(g.Textf("Enable reconcile for %s in %s?", featureName, envName)),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Enable reconcile")),
				h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func redirectField(url string) g.Node {
	if url == "" {
		return nil
	}
	return h.Input(h.Type("hidden"), h.Name("redirect"), h.Value(url))
}
