// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Aspect + font-relative geometry proofs. Every case asserts an EXACT resolved
// rect (per feedback-precise-bounds-tests: assert exact positions, not "drew
// something") at MULTIPLE object sizes and MULTIPLE glyph heights.

package skin_test

import (
	"fmt"
	"testing"

	"github.com/go-widgets/skin"
	"github.com/go-widgets/toolkit"
)

// R is a terse rect constructor for the tables below.
func R(x, y, w, h int) toolkit.Rect { return toolkit.Rect{X: x, Y: y, W: w, H: h} }

// loadInline parses a document from a literal string and instantiates coll.
func loadInline(t *testing.T, src, coll string) *skin.Object {
	t.Helper()
	th, err := skin.Load([]byte(src))
	if err != nil {
		t.Fatalf("load inline: %v\nsrc=%s", err, src)
	}
	o, err := th.New(coll)
	if err != nil {
		t.Fatalf("new %q: %v", coll, err)
	}
	return o
}

// withGlyphHeight runs fn with the active font scaled so GlyphHeight()==gh
// (gh must be a multiple of the 5x7 base height, 7), restoring the default
// afterward. It lets a single table prove font-INVARIANCE of aspect geometry
// and font-RELATIVE resolution of em offsets.
func withGlyphHeight(t *testing.T, gh int, fn func()) {
	t.Helper()
	toolkit.SetFont(toolkit.NewBitmapFont(gh / 7))
	defer toolkit.SetFont(nil)
	if toolkit.GlyphHeight() != gh {
		t.Fatalf("setup: GlyphHeight()=%d, want %d", toolkit.GlyphHeight(), gh)
	}
	fn()
}

// aspectDoc exercises every aspect_mode. Full-box parts (no rel1/rel2) let the
// object bounds be the allotted rect directly; "zero" carves a degenerate
// (zero-width) allotted box to hit applyAspect's guard.
const aspectDoc = `{"collections":{"asp":{"parts":[
  {"name":"both","aspect_mode":"both","aspect":{"pref":1},"align":[0.5,0.5],"states":{"default":{"color":"@surface"}},"type":"rect"},
  {"name":"horiz","aspect_mode":"horizontal","aspect":{"pref":2},"align":[0.5,0],"states":{"default":{"color":"@surface"}},"type":"rect"},
  {"name":"vert","aspect_mode":"vertical","aspect":{"pref":2},"align":[0,0.5],"states":{"default":{"color":"@surface"}},"type":"rect"},
  {"name":"neither","aspect_mode":"neither","aspect":{"min":1,"max":2},"align":[0.5,0.5],"states":{"default":{"color":"@surface"}},"type":"rect"},
  {"name":"zero","aspect_mode":"both","aspect":{"pref":1},"align":[0.5,0.5],"rel1":{"relative":[0.5,0]},"rel2":{"relative":[0.5,1]},"states":{"default":{"color":"@surface"}},"type":"rect"}
]}}}`

func TestSolveAspectModes(t *testing.T) {
	cases := []struct {
		part   string
		bounds toolkit.Rect
		want   toolkit.Rect
	}{
		// both (contain, pref 1:1, centred): the two branches of the fit test.
		{"both", R(0, 0, 40, 20), R(10, 0, 20, 20)}, // wide box → width shrinks
		{"both", R(0, 0, 20, 40), R(0, 10, 20, 20)}, // tall box → height shrinks
		{"both", R(5, 7, 40, 20), R(15, 7, 20, 20)}, // origin offset carried through
		// horizontal (height authoritative, width = h*2, centred x).
		{"horiz", R(0, 0, 40, 20), R(0, 0, 40, 20)},   // 20*2 == allotted width
		{"horiz", R(0, 0, 100, 10), R(40, 0, 20, 10)}, // 10*2 < 100, centred
		// vertical (width authoritative, height = w/2, centred y).
		{"vert", R(0, 0, 40, 20), R(0, 0, 40, 20)},   // 40/2 == allotted height
		{"vert", R(0, 0, 40, 100), R(0, 40, 40, 20)}, // 40/2 < 100, centred
		// neither (clamp own ratio into [1,2]): the three range branches.
		{"neither", R(0, 0, 30, 20), R(0, 0, 30, 20)},  // 1.5 in range → unchanged
		{"neither", R(0, 0, 60, 20), R(10, 0, 40, 20)}, // 3.0 > max → width clamps
		{"neither", R(0, 0, 20, 40), R(0, 10, 20, 20)}, // 0.5 < min → height clamps
		// degenerate allotted box → returned untouched (guard).
		{"zero", R(0, 0, 40, 20), R(20, 0, 0, 20)},
	}
	// Aspect geometry is font-INDEPENDENT: the SAME rects must resolve at every
	// glyph height.
	for _, gh := range []int{7, 14, 21} {
		withGlyphHeight(t, gh, func() {
			for _, c := range cases {
				o := loadInline(t, aspectDoc, "asp")
				o.SetBounds(c.bounds)
				got, ok := o.PartRect(c.part)
				rectEq(t, fmt.Sprintf("gh%d/%s@%v", gh, c.part, c.bounds), got, ok, c.want)
			}
		})
	}
}

// TestSolveFractionalAnchors proves a fractional rel_pos (relative) resolves
// exactly at multiple sizes: the check box is pinned to the vertical CENTRE
// (relative [0,0.5]) so its top tracks (H-12)/2 as the object grows.
func TestSolveFractionalAnchors(t *testing.T) {
	cases := []struct {
		bounds toolkit.Rect
		want   toolkit.Rect
	}{
		{R(0, 0, 120, 20), R(0, 4, 12, 12)},  // (20-12)/2 = 4
		{R(0, 0, 120, 40), R(0, 14, 12, 12)}, // (40-12)/2 = 14
		{R(3, 5, 200, 51), R(3, 24, 12, 12)}, // 5 + (51-12)/2 = 5+19
	}
	for _, c := range cases {
		o := loadCollection(t, "check.skin.json", "check")
		o.SetBounds(c.bounds)
		got, ok := o.PartRect("box")
		rectEq(t, fmt.Sprintf("box@%v", c.bounds), got, ok, c.want)
	}
}

// TestSolveSwitchScales proves the re-authored (aspect-driven) switch knob is
// a CONTAINED square that seats left in Off and right in On, at multiple track
// sizes — the geometry that used to be frozen at 20x20 pixels.
func TestSolveSwitchScales(t *testing.T) {
	cases := []struct {
		bounds  toolkit.Rect
		off, on toolkit.Rect
	}{
		{R(0, 0, 44, 24), R(2, 2, 20, 20), R(22, 2, 20, 20)}, // the historical size
		{R(0, 0, 60, 30), R(2, 2, 26, 26), R(32, 2, 26, 26)}, // larger
		{R(5, 7, 80, 36), R(7, 9, 32, 32), R(51, 9, 32, 32)}, // larger + origin offset
		{R(0, 0, 45, 25), R(2, 2, 21, 21), R(22, 2, 21, 21)}, // odd dimensions
	}
	for _, c := range cases {
		o := loadCollection(t, "switch.skin.json", "switch")
		o.SetBounds(c.bounds)
		got, ok := o.PartRect("knob")
		rectEq(t, fmt.Sprintf("knob/off@%v", c.bounds), got, ok, c.off)
		o.SetState("on")
		got, ok = o.PartRect("knob")
		rectEq(t, fmt.Sprintf("knob/on@%v", c.bounds), got, ok, c.on)
		// The track always fills the object regardless of size.
		got, ok = o.PartRect("track")
		rectEq(t, fmt.Sprintf("track@%v", c.bounds), got, ok, c.bounds)
	}
}

// stateRelDoc has a part whose "shift" state OVERRIDES rel1/rel2 (a per-state
// geometry change, as a knob used to do): it keeps effRel's state-override path
// and validateState's state-rel parsing exercised now that the shipped switch
// expresses its slide through align instead.
const stateRelDoc = `{"collections":{"c":{"parts":[
  {"name":"p","type":"rect","rel1":{"relative":[0,0]},"rel2":{"relative":[1,1]},
   "states":{"default":{"color":"@surface"},
             "shift":{"color":"@surface","rel1":{"relative":[0,0],"offset":[4,6]},"rel2":{"relative":[1,1],"offset":[-4,-6]}}}}
]}}}`

func TestSolveStateRelOverride(t *testing.T) {
	o := loadInline(t, stateRelDoc, "c")
	o.SetBounds(R(0, 0, 40, 20))
	got, ok := o.PartRect("p")
	rectEq(t, "p/default", got, ok, R(0, 0, 40, 20))
	o.SetState("shift")
	got, ok = o.PartRect("p")
	rectEq(t, "p/shift", got, ok, R(4, 6, 32, 8)) // (0+4,0+6)..(40-4,20-6)
}

// TestSolveCardFontRelative proves the re-authored card's strips/dividers/body
// resolve to EXACT rects driven by the active font's glyph height (offset_em),
// at THREE glyph heights — the geometry that used to be frozen at the 5x7 font
// and made the card test skip on GlyphHeight()!=7.
func TestSolveCardFontRelative(t *testing.T) {
	const W, H = 160, 100
	for _, gh := range []int{7, 14, 21} {
		withGlyphHeight(t, gh, func() {
			o := loadCollection(t, "card.skin.json", "card")
			o.SetBounds(R(0, 0, W, H))
			strip := gh + 12 // CardHeaderH == GlyphHeight() + 2*CardPadY(6)
			check := func(name string, want toolkit.Rect) {
				got, ok := o.PartRect(name)
				rectEq(t, fmt.Sprintf("gh%d/%s", gh, name), got, ok, want)
			}
			check("header", R(0, 0, W, strip))
			check("title", R(8, 0, W-8, strip))
			check("hdiv", R(0, strip, W, 1))
			check("footer", R(0, H-strip, W, strip))
			check("fdiv", R(0, H-strip-1, W, 1))
			check("body", R(8, gh+19, W-8, H-(gh+19)))
			check("surface", R(0, 0, W, H))
			check("outline", R(0, 0, W, H))
		})
	}
}
