// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-widgets/toolkit"
)

// colorSpec is a parsed, not-yet-resolved colour value. A value is EITHER a
// palette token (resolved against a *toolkit.Theme at draw time, so a single
// document re-themes for free) OR a literal RGBA baked in at parse time.
//
// JSON surface (see [parseColor]):
//
//	"@surface"           a canonical palette token
//	"@accent_fg"         the accent-foreground token
//	"@muted_face"        a computed (disabled) token
//	"@extra:success"     an escape hatch into Theme.Extra (GTK @define-color)
//	"#RRGGBB"            an opaque hex literal
//	"#RRGGBBAA"          a hex literal with alpha
//	[34,132,228]         an [r,g,b] literal (alpha 255)
//	[34,132,228,128]     an [r,g,b,a] literal
type colorSpec struct {
	// token is the palette token name (canonical or "extra:<name>"), lower-cased
	// with underscores stripped for the canonical set. Empty when literal.
	token string
	// lit is the baked literal colour, valid only when token == "".
	lit toolkit.RGBA
}

// canonicalTokens maps the recognised canonical palette tokens (with any
// underscores already stripped) to the field they resolve to. mutedFace,
// mutedInk and accentFg are computed rather than plain fields, handled in
// [colorSpec.resolve].
var canonicalTokens = map[string]struct{}{
	"background": {}, "surface": {}, "surfacealt": {},
	"onbackground": {}, "onsurface": {}, "accent": {}, "border": {},
	"accentfg": {}, "mutedface": {}, "mutedink": {},
}

// parseColor turns one JSON colour value (a string token/hex, or an [r,g,b(,a)]
// array) into a colorSpec, returning a rich error naming what was wrong. The
// where string is prepended to every error so a bad value is traced to its
// exact collection/part/state slot.
func parseColor(where string, raw json.RawMessage) (colorSpec, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return colorSpec{}, fmt.Errorf("%s: empty colour value", where)
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return colorSpec{}, fmt.Errorf("%s: %w", where, err)
		}
		return parseColorString(where, s)
	case '[':
		var arr []int
		if err := json.Unmarshal(raw, &arr); err != nil {
			return colorSpec{}, fmt.Errorf("%s: colour array: %w", where, err)
		}
		return parseColorArray(where, arr)
	default:
		return colorSpec{}, fmt.Errorf("%s: colour must be a \"@token\"/\"#hex\" string or an [r,g,b(,a)] array, got %s", where, trimmed)
	}
}

// parseColorString handles the string forms: a "@token" or a "#hex" literal.
func parseColorString(where, s string) (colorSpec, error) {
	switch {
	case strings.HasPrefix(s, "@"):
		name := s[1:]
		if strings.HasPrefix(name, "extra:") {
			key := name[len("extra:"):]
			if key == "" {
				return colorSpec{}, fmt.Errorf("%s: @extra: needs a key (e.g. @extra:success_color)", where)
			}
			return colorSpec{token: "extra:" + key}, nil
		}
		norm := strings.ToLower(strings.ReplaceAll(name, "_", ""))
		if _, ok := canonicalTokens[norm]; !ok {
			return colorSpec{}, fmt.Errorf("%s: unknown colour token %q (canonical tokens or @extra:<name>)", where, s)
		}
		return colorSpec{token: norm}, nil
	case strings.HasPrefix(s, "#"):
		return parseHex(where, s)
	default:
		return colorSpec{}, fmt.Errorf("%s: colour string must start with @ (token) or # (hex), got %q", where, s)
	}
}

// parseHex parses #RRGGBB or #RRGGBBAA into a literal colorSpec.
func parseHex(where, s string) (colorSpec, error) {
	h := s[1:]
	if len(h) != 6 && len(h) != 8 {
		return colorSpec{}, fmt.Errorf("%s: hex colour %q must be #RRGGBB or #RRGGBBAA", where, s)
	}
	v, err := strconv.ParseUint(h, 16, 64)
	if err != nil {
		return colorSpec{}, fmt.Errorf("%s: hex colour %q: %w", where, s, err)
	}
	c := toolkit.RGBA{A: 0xFF}
	if len(h) == 6 {
		c.R, c.G, c.B = uint8(v>>16), uint8(v>>8), uint8(v)
	} else {
		c.R, c.G, c.B, c.A = uint8(v>>24), uint8(v>>16), uint8(v>>8), uint8(v)
	}
	return colorSpec{lit: c}, nil
}

// parseColorArray parses an [r,g,b] or [r,g,b,a] integer array into a literal.
func parseColorArray(where string, arr []int) (colorSpec, error) {
	if len(arr) != 3 && len(arr) != 4 {
		return colorSpec{}, fmt.Errorf("%s: colour array needs 3 (rgb) or 4 (rgba) elements, got %d", where, len(arr))
	}
	for _, v := range arr {
		if v < 0 || v > 255 {
			return colorSpec{}, fmt.Errorf("%s: colour component %d out of range 0..255", where, v)
		}
	}
	c := toolkit.RGBA{R: uint8(arr[0]), G: uint8(arr[1]), B: uint8(arr[2]), A: 0xFF}
	if len(arr) == 4 {
		c.A = uint8(arr[3])
	}
	return colorSpec{lit: c}, nil
}

// magenta is the loud sentinel a missing @extra token resolves to, so an
// unresolved reference is glaringly visible on screen rather than silently
// transparent. (Canonical tokens can never be missing; only Extra can.)
var magenta = toolkit.RGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 0xFF}

// resolve turns the spec into a concrete colour against th's palette. Literals
// pass straight through; tokens read th's fields (or its Extra map). The
// computed tokens replicate the toolkit's own disabled-tint and accent-fg maths
// EXACTLY (see mutedInk/mutedFace/accentFg in go-widgets/toolkit) so a skinned
// widget's disabled/prominent faces are byte-identical to the hand-coded one.
func (cs colorSpec) resolve(th *toolkit.Theme) toolkit.RGBA {
	if cs.token == "" {
		return cs.lit
	}
	if strings.HasPrefix(cs.token, "extra:") {
		key := cs.token[len("extra:"):]
		if th.Extra != nil {
			if c, ok := th.Extra[key]; ok {
				return c
			}
		}
		return magenta
	}
	switch cs.token {
	case "background":
		return th.Background
	case "surface":
		return th.Surface
	case "surfacealt":
		return th.SurfaceAlt
	case "onbackground":
		return th.OnBackground
	case "onsurface":
		return th.OnSurface
	case "accent":
		return th.Accent
	case "border":
		return th.Border
	case "accentfg":
		if th.Extra != nil {
			if c, ok := th.Extra["accent_fg_color"]; ok {
				return c
			}
		}
		return toolkit.RGB(0xFF, 0xFF, 0xFF)
	case "mutedface":
		return blend(th.SurfaceAlt, th.Background, 0.5)
	default: // "mutedink" — the map guarantees no other value reaches here.
		return blend(th.OnSurface, th.Background, 0.5)
	}
}

// blend mixes a toward b, with t the weight of a — the exact formula the
// toolkit uses for its muted (disabled) tints (blendRGBA in scrollbar.go),
// reproduced here so skin's @muted_* tokens match to the byte.
func blend(a, b toolkit.RGBA, t float64) toolkit.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*t + float64(y)*(1-t) + 0.5) }
	return toolkit.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

// lerpColor linearly interpolates every channel (including alpha) from a to b
// by p in [0,1] — the per-tick blend a colour transition paints while an
// [github.com/go-widgets/toolkit/anim] animation runs between two states.
func lerpColor(a, b toolkit.RGBA, p float64) toolkit.RGBA {
	li := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*p + 0.5) }
	return toolkit.RGBA{R: li(a.R, b.R), G: li(a.G, b.G), B: li(a.B, b.B), A: li(a.A, b.A)}
}
