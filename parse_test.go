// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin_test

import (
	"os"
	"strings"
	"testing"

	"github.com/go-widgets/skin"
)

func TestLoadValidTestdata(t *testing.T) {
	files := map[string][]string{
		"button.skin.json": {"button"},
		"switch.skin.json": {"switch"},
		"check.skin.json":  {"check"},
		"chip.skin.json":   {"chip"},
		"card.skin.json":   {"card"},
		"demo.skin.json":   {"badge"},
	}
	for f, colls := range files {
		src, err := os.ReadFile("testdata/" + f)
		if err != nil {
			t.Fatal(err)
		}
		th, err := skin.Load(src)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got := th.Collections()
		if len(got) != len(colls) {
			t.Fatalf("%s: collections = %v, want %v", f, got, colls)
		}
		for _, c := range colls {
			if _, err := th.New(c); err != nil {
				t.Fatalf("%s: New(%q): %v", f, c, err)
			}
		}
	}
}

// A minimally valid document, and no-min variant, both parse.
func TestLoadMinimal(t *testing.T) {
	for _, src := range []string{
		`{"collections":{"c":{"min":{"w":10,"h":10},"parts":[{"name":"p","type":"rect","states":{"default":{"color":"@surface"}}}]}}}`,
		`{"collections":{"c":{"parts":[{"name":"p","type":"text","text":"hi","states":{"default":{"ink":"@on_surface"}}}]}}}`,
		// border_width (positive) is set, not rejected.
		`{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"@surface","border":"@border","border_width":2}}}]}}}`,
		// aspect_mode "none" with an aspect block is accepted (block ignored).
		`{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"none","aspect":{"pref":1},"states":{"default":{"color":"@surface"}}}]}}}`,
		// an aspect block with no explicit aspect_mode defaults to "both".
		`{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect":{"pref":1.5},"states":{"default":{"color":"@surface"}}}]}}}`,
		// neither with a valid min<=max range.
		`{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"neither","aspect":{"min":1,"max":2},"states":{"default":{"color":"@surface"}}}]}}}`,
		// offset_em is a recognised endpoint field (font-relative inset).
		`{"collections":{"c":{"parts":[{"name":"p","type":"rect","rel1":{"offset_em":[0,1]},"states":{"default":{"color":"@surface"}}}]}}}`,
	} {
		if _, err := skin.Load([]byte(src)); err != nil {
			t.Fatalf("valid doc rejected: %v\nsrc=%s", err, src)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"invalid json", `{`, "parse JSON"},
		{"unknown top field", `{"collections":{},"bogus":1}`, "unknown field"},
		{"no collections", `{}`, "no collections"},
		{"empty collections", `{"collections":{}}`, "no collections"},
		{"null collection", `{"collections":{"c":null}}`, "is null"},
		{"neg min", `{"collections":{"c":{"min":{"w":-1,"h":2},"parts":[{"name":"p","type":"rect","states":{"default":{}}}]}}}`, "min size cannot be negative"},
		{"no parts", `{"collections":{"c":{"parts":[]}}}`, "has no parts"},
		{"null part", `{"collections":{"c":{"parts":[null]}}}`, "is null"},
		{"empty part name", `{"collections":{"c":{"parts":[{"name":"","type":"rect","states":{"default":{}}}]}}}`, "empty name"},
		{"dup part", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}},{"name":"p","type":"rect","states":{"default":{}}}]}}}`, "duplicate part name"},
		{"unknown type", `{"collections":{"c":{"parts":[{"name":"p","type":"blob","states":{"default":{}}}]}}}`, "unknown type"},
		{"unknown field in part", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","bogus":1,"states":{"default":{}}}]}}}`, "unknown field"},
		{"rel1 to forward", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","rel1":{"to":"ghost"},"states":{"default":{}}}]}}}`, "not an earlier part"},
		{"rel2 to forward", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","rel2":{"to":"ghost"},"states":{"default":{}}}]}}}`, "not an earlier part"},
		{"state rel2 to unknown", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"rel2":{"to":"ghost"}}}}]}}}`, "not an earlier part"},
		{"no states", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{}}]}}}`, "no states"},
		{"missing default", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"hover":{}}}]}}}`, "missing required \"default\""},
		{"null state", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":null}}]}}}`, "is null"},
		{"bad color token", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"@nope"}}}]}}}`, "unknown colour token"},
		{"bad ink token", `{"collections":{"c":{"parts":[{"name":"p","type":"text","states":{"default":{"ink":"@nope"}}}]}}}`, "unknown colour token"},
		{"bad border token", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"border":"@nope"}}}]}}}`, "unknown colour token"},
		{"neg border width", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"border_width":-1}}}]}}}`, "border_width cannot be negative"},
		{"bad radius string", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"radius":"square"}}}]}}}`, "only \"pill\" or a number"},
		{"neg radius", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"radius":-1}}}]}}}`, "radius cannot be negative"},
		{"radius wrong type", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"radius":true}}}]}}}`, "radius"},
		{"colour not prefixed", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"hello"}}}]}}}`, "must start with @"},
		{"hex bad len", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"#123"}}}]}}}`, "must be #RRGGBB"},
		{"hex bad digits", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"#gggggg"}}}]}}}`, "hex colour"},
		{"colour array short", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":[1,2]}}}]}}}`, "needs 3"},
		{"colour array oor", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":[300,1,1]}}}]}}}`, "out of range"},
		{"colour array bad elem", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":[1,2,"x"]}}}]}}}`, "colour array"},
		{"colour wrong type", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":5}}}]}}}`, "colour must be"},
		{"extra empty key", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"@extra:"}}}]}}}`, "needs a key"},
		{"state rel to unknown", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"rel1":{"to":"ghost"}}}}]}}}`, "not an earlier part"},
		{"program null", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[null]}}}`, "is null"},
		{"program empty on", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"","emit":"x"}]}}}`, "empty `on`"},
		{"program neg in", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s","in":-1,"emit":"x"}]}}}`, "cannot be negative"},
		{"program bad ease", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s","ease":"boing","emit":"x"}]}}}`, "unknown ease"},
		{"program target no to", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s","target":["p"]}]}}}`, "no `to` state"},
		{"program target unknown", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s","target":["ghost"],"to":"default"}]}}}`, "not a part"},
		{"program target no state", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s","target":["p"],"to":"hover"}]}}}`, "has no state"},
		{"program does nothing", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{}}}],"programs":[{"on":"s"}]}}}`, "does nothing"},
		{"unknown aspect_mode", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"boing","states":{"default":{}}}]}}}`, "unknown aspect_mode"},
		{"aspect mode no block", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"both","states":{"default":{}}}]}}}`, "needs an `aspect` block"},
		{"neither missing max", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"neither","aspect":{"min":1},"states":{"default":{}}}]}}}`, "needs both min and max"},
		{"neither missing min", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"neither","aspect":{"max":2},"states":{"default":{}}}]}}}`, "needs both min and max"},
		{"neither non-positive", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"neither","aspect":{"min":0,"max":2},"states":{"default":{}}}]}}}`, "ratios must be positive"},
		{"neither min gt max", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"neither","aspect":{"min":3,"max":2},"states":{"default":{}}}]}}}`, "cannot exceed"},
		{"pref missing", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"both","aspect":{"min":1,"max":2},"states":{"default":{}}}]}}}`, "needs a `pref` ratio"},
		{"pref non-positive", `{"collections":{"c":{"parts":[{"name":"p","type":"rect","aspect_mode":"horizontal","aspect":{"pref":0},"states":{"default":{}}}]}}}`, "ratios must be positive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := skin.Load([]byte(c.src))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), c.want)
			}
		})
	}
}
