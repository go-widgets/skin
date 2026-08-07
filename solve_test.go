// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin_test

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// rectEq asserts a part resolves to an EXACT rect (per feedback-precise-bounds:
// assert exact positions, not "something drew").
func rectEq(t *testing.T, name string, got toolkit.Rect, ok bool, want toolkit.Rect) {
	t.Helper()
	if !ok {
		t.Fatalf("%s: part not found", name)
	}
	if got != want {
		t.Fatalf("%s: rect = %+v, want %+v", name, got, want)
	}
}

func TestSolveSwitchGeometry(t *testing.T) {
	o := loadCollection(t, "switch.skin.json", "switch")
	o.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 44, H: 24})

	r, ok := o.PartRect("track")
	rectEq(t, "track", r, ok, toolkit.Rect{X: 0, Y: 0, W: 44, H: 24})
	r, ok = o.PartRect("knob")
	rectEq(t, "knob/off", r, ok, toolkit.Rect{X: 2, Y: 2, W: 20, H: 20})

	o.SetState("on") // knob slides right (per-state geometry override)
	r, ok = o.PartRect("knob")
	rectEq(t, "knob/on", r, ok, toolkit.Rect{X: 22, Y: 2, W: 20, H: 20})

	if _, ok := o.PartRect("ghost"); ok {
		t.Fatal("unknown part should report not-found")
	}
}

func TestSolveCheckGeometry(t *testing.T) {
	o := loadCollection(t, "check.skin.json", "check")
	o.SetBounds(toolkit.Rect{X: 5, Y: 7, W: 120, H: 20})

	// Box is a fixed 12×12, vertically centred: boxY = 7 + (20-12)/2 = 11.
	r, ok := o.PartRect("box")
	rectEq(t, "box", r, ok, toolkit.Rect{X: 5, Y: 11, W: 12, H: 12})

	// First checkmark pixel: box-relative offset (3,6) → (8,17), 1×1.
	r, ok = o.PartRect("ck0a")
	rectEq(t, "ck0a", r, ok, toolkit.Rect{X: 8, Y: 17, W: 1, H: 1})

	// Label anchors to the object with an x offset of 16, spanning to the far
	// corner: (5+16, 7) .. (5+120, 7+20) → {21,7,104,20}.
	r, ok = o.PartRect("label")
	rectEq(t, "label", r, ok, toolkit.Rect{X: 21, Y: 7, W: 104, H: 20})
}

func TestSolveCardGeometry(t *testing.T) {
	if toolkit.GlyphHeight() != 7 {
		t.Skipf("card authored for glyphH=7, got %d", toolkit.GlyphHeight())
	}
	o := loadCollection(t, "card.skin.json", "card")
	o.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 100})

	r, ok := o.PartRect("header")
	rectEq(t, "header", r, ok, toolkit.Rect{X: 0, Y: 0, W: 160, H: 19})
	r, ok = o.PartRect("title") // anchored to header, inset 8 on the left
	rectEq(t, "title", r, ok, toolkit.Rect{X: 8, Y: 0, W: 152, H: 19})
	r, ok = o.PartRect("hdiv")
	rectEq(t, "hdiv", r, ok, toolkit.Rect{X: 0, Y: 19, W: 160, H: 1})
	r, ok = o.PartRect("footer")
	rectEq(t, "footer", r, ok, toolkit.Rect{X: 0, Y: 81, W: 160, H: 19})
	r, ok = o.PartRect("fdiv")
	rectEq(t, "fdiv", r, ok, toolkit.Rect{X: 0, Y: 80, W: 160, H: 1})
	r, ok = o.PartRect("body")
	rectEq(t, "body", r, ok, toolkit.Rect{X: 8, Y: 26, W: 152, H: 74})
	r, ok = o.PartRect("outline")
	rectEq(t, "outline", r, ok, toolkit.Rect{X: 0, Y: 0, W: 160, H: 100})
}
