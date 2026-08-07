// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"math"

	"github.com/go-widgets/toolkit"
)

// fracInt returns the integer contribution of fraction frac over an integer
// span. The three fractions the widget vocabulary actually uses — 0, 0.5, 1 —
// are computed with INTEGER arithmetic (0, span/2, span) so a skinned box
// centres byte-identically to a hand-coded `(H-boxH)/2`; any other fraction
// falls back to a truncating float multiply. Keeping 0.5 == integer division
// (which truncates toward zero, matching Go's `/`) is what makes the parity
// gate pass on odd sizes.
func fracInt(frac float64, span int) int {
	switch frac {
	case 0:
		return 0
	case 1:
		return span
	case 0.5:
		return span / 2
	default:
		return int(float64(span) * frac)
	}
}

// effRel returns the endpoints a part uses in state st: the state's own rel
// override when present, otherwise the part's default endpoint. A nil st (a
// part with no description for the requested state) uses the part defaults.
func effRel(p *Part, st *State) (relSpec, relSpec) {
	r1, r2 := p.rel1, p.rel2
	if st != nil {
		if st.rel1 != nil {
			r1 = *st.rel1
		}
		if st.rel2 != nil {
			r2 = *st.rel2
		}
	}
	return r1, r2
}

// endpoint resolves one relSpec to an absolute (x, y) against the object rect
// obj and the already-resolved sibling rects in anchors.
func endpoint(rs relSpec, obj toolkit.Rect, anchors map[string]toolkit.Rect) (int, int) {
	base := obj
	if rs.to != "" {
		base = anchors[rs.to]
	}
	x := base.X + fracInt(rs.relx, base.W) + rs.offx
	y := base.Y + fracInt(rs.rely, base.H) + rs.offy
	return x, y
}

// resolveRect resolves a part's rectangle in state st: rel1 gives the top-left,
// rel2 the far corner, so the rect is (x1, y1, x2-x1, y2-y1).
func resolveRect(p *Part, st *State, obj toolkit.Rect, anchors map[string]toolkit.Rect) toolkit.Rect {
	r1, r2 := effRel(p, st)
	x1, y1 := endpoint(r1, obj, anchors)
	x2, y2 := endpoint(r2, obj, anchors)
	return toolkit.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// anchorMap resolves every part's rectangle in the state named by stateOf, in
// declaration order, so a part that anchors to an earlier sibling sees that
// sibling's already-resolved rect. It is the stable anchor frame both settled
// and transitioning parts are placed against.
func (c *Collection) anchorMap(obj toolkit.Rect, stateOf func(*Part) *State) map[string]toolkit.Rect {
	anchors := make(map[string]toolkit.Rect, len(c.Parts))
	for _, p := range c.Parts {
		anchors[p.Name] = resolveRect(p, stateOf(p), obj, anchors)
	}
	return anchors
}

// lerpRect linearly interpolates each field of a toward b by prog, rounding to
// the nearest pixel. At prog 0 it is exactly a and at prog 1 exactly b, so a
// finished transition lands on the target rect with no rounding drift.
func lerpRect(a, b toolkit.Rect, prog float64) toolkit.Rect {
	li := func(u, v int) int { return int(math.Round(float64(u) + float64(v-u)*prog)) }
	return toolkit.Rect{X: li(a.X, b.X), Y: li(a.Y, b.Y), W: li(a.W, b.W), H: li(a.H, b.H)}
}
