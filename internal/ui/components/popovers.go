package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Popover renders a native popover dialog: a header row with the title and a
// top-right X close button on the same line, followed by the body nodes. The id
// doubles as the close button's popovertarget. class is applied to the popover
// element when non-empty (e.g. "popover-wide").
func Popover(id, class, title string, body ...g.Node) g.Node {
	attrs := []g.Node{g.Attr("popover", ""), h.ID(id)}
	if class != "" {
		attrs = append(attrs, h.Class(class))
	}
	attrs = append(attrs, popoverHeader(id, title))
	attrs = append(attrs, body...)
	return h.Div(attrs...)
}

// PopoverActions wraps optional footer action buttons (e.g. a submit button) for
// a popover. The X in the header handles dismissal, so a separate cancel button
// is usually unnecessary.
func PopoverActions(buttons ...g.Node) g.Node {
	return h.Div(h.Class("popover-actions"), g.Group(buttons))
}

// PopoverCloseButton renders the X dismiss button targeting the given popover.
func PopoverCloseButton(popoverID string) g.Node {
	return h.Button(h.Type("button"), h.Class("popover-close"),
		g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"),
		g.Attr("aria-label", "Close"), g.Raw("&times;"))
}

func popoverHeader(id, title string) g.Node {
	return h.Div(
		h.Class("popover-header"),
		h.H3(g.Text(title)),
		PopoverCloseButton(id),
	)
}

// RedeployPopover renders a confirmation popover for triggering a redeploy.
// Returns nil if enabled is false. If redirectURL is non-empty, the form
// includes a hidden field so the handler redirects back to that URL.
func RedeployPopover(popoverID, action, featureName, envName string, enabled bool, redirectURL string) g.Node {
	if !enabled {
		return nil
	}
	return Popover(
		popoverID, "", "Confirm redeploy",
		h.Form(
			h.Method("POST"), h.Action(action),
			redirectField(redirectURL),
			h.P(g.Textf("Force a fresh deploy of %s in %s?", featureName, envName)),
			PopoverActions(
				h.Button(h.Type("submit"), g.Text("Trigger redeploy")),
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
		return Popover(
			popoverID, "", "Disable reconcile",
			h.Form(
				h.Method("POST"), h.Action(action),
				h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("false")),
				redirectField(redirectURL),
				h.P(g.Textf("Disable reconcile for %s in %s? Reconciliation will stop until re-enabled.", featureName, envName)),
				h.Label(h.For(reasonID), g.Text("Reason for disabling reconcile")),
				h.Textarea(h.ID(reasonID), h.Name("reason"), g.Attr("maxlength", "1000"), g.Attr("required", ""), h.Rows("3")),
				PopoverActions(
					h.Button(h.Type("submit"), g.Text("Disable reconcile")),
				),
			),
		)
	}

	return Popover(
		popoverID, "", "Enable reconcile",
		h.Form(
			h.Method("POST"), h.Action(action),
			h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("true")),
			redirectField(redirectURL),
			h.P(g.Textf("Enable reconcile for %s in %s?", featureName, envName)),
			PopoverActions(
				h.Button(h.Type("submit"), g.Text("Enable reconcile")),
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
