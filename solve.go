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
// obj and the already-resolved sibling rects in anchors. The fractional
// position (relx/rely) and pixel offset are joined by the font-relative offset
// (emx/emy), resolved against the active font's glyph height so a text-driven
// inset scales with the font.
func endpoint(rs relSpec, obj toolkit.Rect, anchors map[string]toolkit.Rect) (int, int) {
	base := obj
	if rs.to != "" {
		base = anchors[rs.to]
	}
	gh := toolkit.GlyphHeight()
	x := base.X + fracInt(rs.relx, base.W) + rs.offx + emPx(rs.emx, gh)
	y := base.Y + fracInt(rs.rely, base.H) + rs.offy + emPx(rs.emy, gh)
	return x, y
}

// emPx converts an em (glyph-height) measure to whole pixels against glyph
// height gh, rounding to nearest. The em==0 fast path keeps the pixel-only
// endpoints (every historical collection) free of any float work, so their
// resolved rects — and the parity gate — stay byte-identical.
func emPx(em float64, gh int) int {
	if em == 0 {
		return 0
	}
	return int(math.Round(em * float64(gh)))
}

// resolveRect resolves a part's rectangle in state st: rel1 gives the top-left,
// rel2 the far corner, so the raw rect is (x1, y1, x2-x1, y2-y1). When the part
// carries an aspect constraint the raw rect becomes the ALLOTTED box and the
// ratio-held rect is positioned inside it by the effective align.
func resolveRect(p *Part, st *State, obj toolkit.Rect, anchors map[string]toolkit.Rect) toolkit.Rect {
	r1, r2 := effRel(p, st)
	x1, y1 := endpoint(r1, obj, anchors)
	x2, y2 := endpoint(r2, obj, anchors)
	r := toolkit.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
	if p.aspect.mode == aspectNone {
		return r
	}
	ax, ay := effAlign(p, st)
	return applyAspect(p.aspect, r, ax, ay)
}

// effAlign returns the alignment a part uses in state st: the state's own align
// override when present, otherwise the part's default align. It drives BOTH
// text placement and, for an aspect-constrained part, where the ratio-held rect
// seats inside its allotted box.
func effAlign(p *Part, st *State) (float64, float64) {
	a := p.align
	if st != nil && st.align != nil {
		a = *st.align
	}
	return a[0], a[1]
}

// applyAspect shrinks the allotted rect r to satisfy aspect a, then seats the
// result inside r by (alignX, alignY). A degenerate allotted rect (non-positive
// side) is returned untouched — there is no ratio to hold.
func applyAspect(a aspectSpec, r toolkit.Rect, alignX, alignY float64) toolkit.Rect {
	if r.W <= 0 || r.H <= 0 {
		return r
	}
	w0, h0 := r.W, r.H
	var w, h int
	switch a.mode {
	case aspectHorizontal: // height authoritative → width = h*pref
		w, h = int(math.Round(float64(h0)*a.pref)), h0
	case aspectVertical: // width authoritative → height = w/pref
		w, h = w0, int(math.Round(float64(w0)/a.pref))
	case aspectBoth: // contain: largest rect of ratio pref fitting inside
		if float64(w0)/float64(h0) > a.pref {
			w, h = int(math.Round(float64(h0)*a.pref)), h0
		} else {
			w, h = w0, int(math.Round(float64(w0)/a.pref))
		}
	default: // aspectNeither: clamp the rect's own ratio into [min,max]
		switch cur := float64(w0) / float64(h0); {
		case cur > a.max:
			w, h = int(math.Round(float64(h0)*a.max)), h0
		case cur < a.min:
			w, h = w0, int(math.Round(float64(w0)/a.min))
		default:
			w, h = w0, h0
		}
	}
	return toolkit.Rect{
		X: r.X + fracInt(alignX, w0-w),
		Y: r.Y + fracInt(alignY, h0-h),
		W: w,
		H: h,
	}
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
