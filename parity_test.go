// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin_test

import (
	"os"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/skin"
	"github.com/go-widgets/toolkit"
)

// mapSource is a trivial [skin.TextSource] for tests.
type mapSource map[string]string

func (m mapSource) Get(p string) (string, bool) { v, ok := m[p]; return v, ok }

// renderWidget paints w at bounds into a fresh canvas sized to bounds + margin,
// returning the RGBA bytes. The margin (placing the widget away from 0,0)
// catches any off-by-one at the surface edge.
func renderWidget(w toolkit.Widget, bounds toolkit.Rect, th *toolkit.Theme) []byte {
	const margin = 12
	cw := bounds.X + bounds.W + margin
	ch := bounds.Y + bounds.H + margin
	buf := make([]byte, cw*ch*4)
	p := painter.NewPixelPainter(buf, cw, ch)
	w.SetBounds(bounds)
	w.Draw(p, th)
	return buf
}

// diffPixels returns the (x,y) and both RGBA of the first differing pixel, or
// ok=false when the two buffers are byte-identical.
func diffPixels(a, b []byte, stride int) (x, y int, av, bv [4]byte, ok bool) {
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] || a[i+3] != b[i+3] {
			px := (i / 4) % stride
			py := (i / 4) / stride
			return px, py, [4]byte{a[i], a[i+1], a[i+2], a[i+3]}, [4]byte{b[i], b[i+1], b[i+2], b[i+3]}, true
		}
	}
	return 0, 0, av, bv, false
}

// assertParity renders the hand-coded widget and the skinned object at the same
// bounds and asserts byte-identical output (the AA maths is deterministic, so
// no tolerance is needed when the vocabulary matches).
func assertParity(t *testing.T, name string, want toolkit.Widget, got *skin.Object, bounds toolkit.Rect, th *toolkit.Theme) {
	t.Helper()
	stride := bounds.X + bounds.W + 12
	a := renderWidget(want, bounds, th)
	b := renderWidget(got, bounds, th)
	if x, y, av, bv, differ := diffPixels(a, b, stride); differ {
		t.Fatalf("%s: pixel mismatch at (%d,%d): toolkit=%v skin=%v", name, x, y, av, bv)
	}
}

func loadCollection(t *testing.T, file, coll string) *skin.Object {
	t.Helper()
	src, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	th, err := skin.Load(src)
	if err != nil {
		t.Fatalf("load %s: %v", file, err)
	}
	o, err := th.New(coll)
	if err != nil {
		t.Fatalf("new %s: %v", coll, err)
	}
	return o
}

func TestParityButton(t *testing.T) {
	th := toolkit.DefaultLight()
	bounds := toolkit.Rect{X: 10, Y: 10, W: 96, H: 28}
	for _, st := range []string{"default", "hover", "pressed", "disabled"} {
		want := &toolkit.Button{Label: "OK", PressFeedback: true}
		switch st {
		case "hover":
			want.SetHovered(true)
		case "pressed":
			want.SetPressed(true)
		case "disabled":
			want.Disabled = true
		}
		got := loadCollection(t, "button.skin.json", "button")
		got.Bind(mapSource{"$.label": "OK"})
		got.SetState(st)
		assertParity(t, "button/"+st, want, got, bounds, th)
	}
}

func TestParitySwitch(t *testing.T) {
	th := toolkit.DefaultLight()
	bounds := toolkit.Rect{X: 10, Y: 10, W: 44, H: 24}
	for _, st := range []string{"default", "on", "disabled"} {
		want := &toolkit.Switch{}
		switch st {
		case "on":
			want.On = true
		case "disabled":
			want.Disabled = true
		}
		got := loadCollection(t, "switch.skin.json", "switch")
		got.SetState(st)
		assertParity(t, "switch/"+st, want, got, bounds, th)
	}
}

func TestParityCheck(t *testing.T) {
	th := toolkit.DefaultLight()
	bounds := toolkit.Rect{X: 10, Y: 10, W: 120, H: 20}
	cases := []struct {
		name    string
		state   string
		checked bool
		dis     bool
	}{
		{"default", "default", false, false},
		{"checked", "checked", true, false},
		{"disabled", "disabled", false, true},
	}
	for _, c := range cases {
		want := &toolkit.CheckButton{Label: "OK", Checked: c.checked}
		want.Disabled = c.dis
		got := loadCollection(t, "check.skin.json", "check")
		got.Bind(mapSource{"$.label": "OK"})
		got.SetState(c.state)
		assertParity(t, "check/"+c.name, want, got, bounds, th)
	}
}

func TestParityChip(t *testing.T) {
	th := toolkit.DefaultLight()
	bounds := toolkit.Rect{X: 10, Y: 10, W: 80, H: 18}
	want := &toolkit.Chip{Text: "tag"}
	got := loadCollection(t, "chip.skin.json", "chip")
	got.Bind(mapSource{"$.label": "tag"})
	assertParity(t, "chip/default", want, got, bounds, th)
}

func TestParityCard(t *testing.T) {
	if toolkit.GlyphHeight() != 7 {
		t.Skipf("card.skin.json is authored for the default 5x7 font (glyphH=7); active font glyphH=%d", toolkit.GlyphHeight())
	}
	th := toolkit.DefaultLight()
	bounds := toolkit.Rect{X: 10, Y: 10, W: 160, H: 100}
	want := &toolkit.Card{Title: "Hello", Body: "world", Footer: "foot"}
	got := loadCollection(t, "card.skin.json", "card")
	got.Bind(mapSource{"$.title": "Hello", "$.body": "world", "$.footer": "foot"})
	assertParity(t, "card/default", want, got, bounds, th)
}
