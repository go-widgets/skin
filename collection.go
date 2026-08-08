// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"sort"

	"github.com/go-widgets/toolkit"
)

// Theme is a parsed+validated set of skin collections together with the palette
// its @-tokens resolve against. It is the output of [Load] and the factory for
// live [Object]s (see [Theme.New]). A Theme is immutable after Load except for
// [Theme.SetPalette]; it holds no per-instance state, so one Theme backs any
// number of concurrent Objects.
type Theme struct {
	collections map[string]*Collection
	names       []string // collection names, sorted, for deterministic listing
	palette     *toolkit.Theme
}

// Palette returns the [toolkit.Theme] this skin's @-tokens resolve against. It
// defaults to [toolkit.DefaultLight]; swap it (or a [toolkit.LoadGTKTheme]
// result, whose Extra map feeds @extra:<name> tokens) via [Theme.SetPalette].
// The returned pointer is the live palette — a caller may pass it straight to
// [Object.Draw].
func (t *Theme) Palette() *toolkit.Theme { return t.palette }

// SetPalette replaces the palette @-tokens resolve against. Passing the result
// of [toolkit.LoadGTKTheme] makes skin a strict superset of that loader: every
// canonical field plus every @define-color entry (via @extra:<name>) becomes
// referenceable from a document. A nil p restores [toolkit.DefaultLight].
func (t *Theme) SetPalette(p *toolkit.Theme) {
	if p == nil {
		p = toolkit.DefaultLight()
	}
	t.palette = p
}

// Collections returns the collection names in sorted order — handy for a host
// that enumerates what a loaded document offers.
func (t *Theme) Collections() []string {
	out := make([]string, len(t.names))
	copy(out, t.names)
	return out
}

// Collection is one named widget/layout template: a minimum size, an ordered
// list of parts (drawn back-to-front) and the signal→state programs that drive
// them. Parts resolve their geometry in list order, so a part may only anchor
// to a sibling declared BEFORE it (no forward references, no cycles).
type Collection struct {
	Name       string
	MinW, MinH int
	Parts      []*Part
	Programs   []*Program

	partIndex map[string]int // name → index into Parts
}

// PartType is the kind of primitive a part paints.
type PartType int

const (
	// PartRect is a (optionally rounded, optionally bordered) filled rectangle.
	PartRect PartType = iota
	// PartText draws a string (from a binding or a static literal) in the
	// toolkit's active font, positioned by the part's align within its box.
	PartText
	// PartImage delegates to a host-registered drawable (see
	// [Object.SetImage]) — the "swallow"/external seam for content the
	// rect/text vocabulary can't express (a real vector icon, a photo …).
	PartImage
)

// Part is one element of a collection: a primitive with default geometry, an
// optional text/image source, and a map of per-state descriptions. Every part
// MUST define a "default" state; other states are optional and fall back to
// "default" when the object is in a state the part doesn't describe.
type Part struct {
	Name     string
	Type     PartType
	TextFrom string // mvvm-style binding path ($.title); "" = none
	Text     string // static text fallback when TextFrom is empty/unbound
	Image    string // drawable name for PartImage; "" = none

	rel1, rel2 relSpec    // part-level default geometry
	align      [2]float64 // part-level default content alignment
	aspect     aspectSpec // part-level ratio constraint (aspectNone = off)
	states     map[string]*State
}

// relSpec is one geometry endpoint: a FRACTIONAL position across some reference
// box (relx/rely, 0..1) plus a pixel offset (offx/offy) and a FONT-RELATIVE
// offset (emx/emy, in em = glyph-height units, resolved against the active
// font's [github.com/go-widgets/toolkit.GlyphHeight] at draw time). The
// reference is the object itself (to == "") or a sibling part (to == that
// part's name). The endpoint is
//
//	base.origin + rel_pos*base.size + offset_px + round(offset_em*glyphH)
//
// Pixel-only authoring is the zero case (emx==emy==0), which reproduces the
// historical behaviour byte-for-byte; a text-driven part instead expresses its
// insets in em so it tracks the font instead of assuming the default 5x7.
type relSpec struct {
	to         string
	relx, rely float64
	offx, offy int
	emx, emy   float64
}

// aspectMode selects how (if at all) a part is held to a width:height ratio
// within the rect its rel1/rel2 carve out — the Edje `aspect_mode` idea. The
// zero value is aspectNone (unconstrained), so a part that names no aspect
// keeps its raw rel1/rel2 rect exactly as before.
type aspectMode int

const (
	// aspectNone applies no ratio constraint (the raw rel1/rel2 rect is used).
	aspectNone aspectMode = iota
	// aspectNeither keeps the rect's own ratio when it already lies within
	// [min,max]; otherwise it shrinks the offending dimension until the ratio
	// reaches the nearest bound (Edje NEITHER: respect the range, force nothing).
	aspectNeither
	// aspectHorizontal treats HEIGHT as authoritative and derives width =
	// round(height*pref): the part scales horizontally to hold the ratio.
	aspectHorizontal
	// aspectVertical treats WIDTH as authoritative and derives height =
	// round(width/pref): the part scales vertically to hold the ratio.
	aspectVertical
	// aspectBoth fits the largest rect of ratio pref that CONTAINS within the
	// allotted rect (neither dimension may grow) — the classic square-knob case.
	aspectBoth
)

// aspectSpec is a part's ratio constraint. pref is the target width:height for
// the horizontal/vertical/both modes; min/max bound the accepted ratio for the
// neither mode. The resolved rect is positioned inside its allotted box by the
// part's (or state's) align, so align doubles as the aspect anchor — align
// [0,0.5] pins a contained square to the left edge (a switch knob's Off seat),
// [1,0.5] to the right (its On seat).
type aspectSpec struct {
	mode           aspectMode
	min, max, pref float64
}

// State is one part description: the visual it paints while the part is in this
// state, plus optional geometry/alignment overrides (so a part can MOVE or
// RESIZE between states — how a switch knob slides). Any unset visual inherits
// nothing; it simply isn't painted (an unset fill draws no body, an unset
// border draws no stroke).
type State struct {
	Name string

	hasFill   bool
	fill      colorSpec
	hasInk    bool
	ink       colorSpec
	hasBorder bool
	border    colorSpec
	borderW   int
	radius    radiusSpec
	visible   bool

	rel1, rel2 *relSpec    // nil = inherit the part's default endpoint
	align      *[2]float64 // nil = inherit the part's default alignment
}

// radiusSpec is a corner radius: a fixed pixel count, or "pill" meaning
// half the shorter side of the resolved rect (a full pill/circle). The painter
// clamps any radius to half the shorter side anyway, but resolving "pill" to a
// concrete value keeps radius interpolation well-defined during transitions.
type radiusSpec struct {
	pill bool
	px   int
}

// resolve returns the concrete pixel radius for a rect of size w×h.
func (rs radiusSpec) resolve(w, h int) int {
	if rs.pill {
		m := w
		if h < m {
			m = h
		}
		return m / 2
	}
	return rs.px
}

// Program is one signal→state rule: when signal On is received, every part in
// Target transitions To the named state over In seconds shaped by Ease, and (if
// Emit is set) a signal Emit is raised outward. A Target may be empty (a pure
// emitter). In == 0 snaps instantly with no animation.
type Program struct {
	On     string
	Target []string
	To     string
	In     float64
	Ease   toolkit.Easing
	Emit   string
}

// sortedNames returns the keys of m in sorted order (deterministic iteration).
func sortedNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
