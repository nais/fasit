package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Emphasis classifies a table cell relative to its column's dominant value, for
// "spot the odd one out" highlighting in columns where values are normally
// expected to match (config values across environments, deployed versions, ...).
type Emphasis int

const (
	// EmphasisNone applies no highlighting: used when a column has no clear norm
	// (e.g. a tie for the most frequent value) so nothing should be singled out.
	EmphasisNone Emphasis = iota
	// EmphasisConsensus marks a cell matching the column's dominant value; it is
	// rendered muted so it recedes.
	EmphasisConsensus
	// EmphasisOutlier marks a cell that differs from the dominant value; it is
	// accented so it stands out.
	EmphasisOutlier
)

// ColumnConsensus classifies each value (in row order) against the column's
// dominant (most frequent) value:
//
//   - a uniform column (one distinct value) -> every cell EmphasisConsensus;
//   - a clear dominant value -> dominant cells EmphasisConsensus, the rest
//     EmphasisOutlier;
//   - no strict winner (a tie for the top count) -> every cell EmphasisNone, so
//     we never highlight arbitrarily.
//
// Empty strings are treated as ordinary values.
func ColumnConsensus(values []string) []Emphasis {
	out := make([]Emphasis, len(values))
	if len(values) < 2 {
		return out
	}

	counts := make(map[string]int, len(values))
	for _, v := range values {
		counts[v]++
	}
	if len(counts) == 1 {
		for i := range out {
			out[i] = EmphasisConsensus
		}
		return out
	}

	dominant, top, tie := "", 0, false
	for v, c := range counts {
		switch {
		case c > top:
			dominant, top, tie = v, c, false
		case c == top:
			tie = true
		}
	}
	if tie {
		return out
	}

	for i, v := range values {
		if v == dominant {
			out[i] = EmphasisConsensus
		} else {
			out[i] = EmphasisOutlier
		}
	}
	return out
}

// ConsensusCell wraps a cell's content with the emphasis decided by
// ColumnConsensus. Consensus values are muted so the column recedes; outliers
// keep full-strength text so they stand out by contrast. EmphasisNone leaves the
// content untouched.
func ConsensusCell(e Emphasis, content ...g.Node) g.Node {
	switch e {
	case EmphasisOutlier:
		return h.Span(append([]g.Node{h.Class("cell-outlier")}, content...)...)
	case EmphasisConsensus:
		return h.Span(append([]g.Node{h.Class("cell-consensus")}, content...)...)
	default:
		return g.Group(content)
	}
}
