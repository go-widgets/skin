// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// parseColor's empty-value guard cannot be reached through Load (a present JSON
// field always has non-empty bytes), so it is exercised directly here.
func TestParseColorEmptyRaw(t *testing.T) {
	if _, err := parseColor("where", []byte("   ")); err == nil {
		t.Fatal("expected error for empty colour value")
	}
}

// parseColor's string-unmarshal error is unreachable through Load (the outer
// decode guarantees a valid JSON string), so it is triggered directly with an
// invalid escape.
func TestParseColorBadStringEscape(t *testing.T) {
	if _, err := parseColor("where", []byte(`"\x"`)); err == nil {
		t.Fatal("expected unmarshal error for invalid escape")
	}
}

// The 8-digit hex form (#RRGGBBAA) carries alpha.
func TestParseHex8(t *testing.T) {
	cs, err := parseColor("where", []byte(`"#11223344"`))
	if err != nil {
		t.Fatal(err)
	}
	want := toolkit.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	if cs.lit != want {
		t.Fatalf("hex8 = %v, want %v", cs.lit, want)
	}
}

// blend's t<0 / t>1 clamps are defensive (resolve only ever passes 0.5), so
// they are asserted directly.
func TestBlendClamp(t *testing.T) {
	a := toolkit.RGB(10, 20, 30)
	b := toolkit.RGB(40, 50, 60)
	if got := blend(a, b, -1); got != b { // t<0 → all of b
		t.Fatalf("blend(-1) = %v, want %v", got, b)
	}
	if got := blend(a, b, 2); got != a { // t>1 → all of a (but alpha forced 255)
		t.Fatalf("blend(2) = %v, want %v", got, a)
	}
}

func TestColorResolveAllTokens(t *testing.T) {
	th := toolkit.DefaultLight()
	th.Extra = map[string]toolkit.RGBA{
		"accent_fg_color": toolkit.RGB(1, 2, 3),
		"warn_color":      toolkit.RGB(9, 8, 7),
	}
	cases := []struct {
		token string
		want  toolkit.RGBA
	}{
		{"background", th.Background},
		{"surface", th.Surface},
		{"surfacealt", th.SurfaceAlt},
		{"onbackground", th.OnBackground},
		{"onsurface", th.OnSurface},
		{"accent", th.Accent},
		{"border", th.Border},
		{"accentfg", toolkit.RGB(1, 2, 3)},
		{"mutedface", blend(th.SurfaceAlt, th.Background, 0.5)},
		{"mutedink", blend(th.OnSurface, th.Background, 0.5)},
		{"extra:warn_color", toolkit.RGB(9, 8, 7)},
	}
	for _, c := range cases {
		if got := (colorSpec{token: c.token}).resolve(th); got != c.want {
			t.Fatalf("token %q = %v, want %v", c.token, got, c.want)
		}
	}
	// A literal passes straight through.
	lit := toolkit.RGB(5, 6, 7)
	if got := (colorSpec{lit: lit}).resolve(th); got != lit {
		t.Fatalf("literal = %v", got)
	}
}

func TestColorResolveFallbacks(t *testing.T) {
	bare := toolkit.DefaultLight() // Extra == nil
	// accentfg with no Extra → white.
	if got := (colorSpec{token: "accentfg"}).resolve(bare); got != toolkit.RGB(0xFF, 0xFF, 0xFF) {
		t.Fatalf("accentfg fallback = %v", got)
	}
	// missing extra → loud magenta sentinel (both nil-Extra and present-but-absent).
	if got := (colorSpec{token: "extra:missing"}).resolve(bare); got != magenta {
		t.Fatalf("missing extra (nil map) = %v", got)
	}
	withMap := toolkit.DefaultLight()
	withMap.Extra = map[string]toolkit.RGBA{"other": toolkit.RGB(1, 1, 1)}
	if got := (colorSpec{token: "extra:missing"}).resolve(withMap); got != magenta {
		t.Fatalf("missing extra (present map) = %v", got)
	}
	if got := (colorSpec{token: "accentfg"}).resolve(withMap); got != toolkit.RGB(0xFF, 0xFF, 0xFF) {
		t.Fatalf("accentfg missing key = %v", got)
	}
}

func TestRadiusResolve(t *testing.T) {
	if got := (radiusSpec{px: 6}).resolve(100, 100); got != 6 {
		t.Fatalf("fixed radius = %d", got)
	}
	if got := (radiusSpec{pill: true}).resolve(40, 20); got != 10 { // h < w → h/2
		t.Fatalf("pill (wide) = %d", got)
	}
	if got := (radiusSpec{pill: true}).resolve(20, 40); got != 10 { // w <= h → w/2
		t.Fatalf("pill (tall) = %d", got)
	}
}

func TestFracIntDefault(t *testing.T) {
	// The 0 / 0.5 / 1 fast paths are covered by the widgets; the float fallback
	// (any other fraction) is exercised here.
	if got := fracInt(0.25, 100); got != 25 {
		t.Fatalf("fracInt(0.25,100) = %d", got)
	}
	if got := fracInt(0.75, 10); got != 7 { // int(7.5) truncates
		t.Fatalf("fracInt(0.75,10) = %d", got)
	}
}
