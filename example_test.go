// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin_test

import (
	"fmt"

	"github.com/go-widgets/skin"
	"github.com/go-widgets/toolkit"
)

// Example loads a one-collection document, instantiates it, drives its state
// machine with a signal and inspects the result — the whole pipeline in
// miniature.
func Example() {
	const doc = `{
	  "collections": {
	    "pill": {
	      "min": {"w": 60, "h": 20},
	      "parts": [
	        {"name": "bg", "type": "rect",
	         "rel1": {}, "rel2": {"relative": [1,1]},
	         "states": {
	           "default": {"color": "@surface", "radius": "pill"},
	           "active":  {"color": "@accent",  "radius": "pill"}
	         }}
	      ],
	      "programs": [
	        {"on": "select", "target": ["bg"], "to": "active", "in": 0, "emit": "selected"}
	      ]
	    }
	  }
	}`

	theme, err := skin.Load([]byte(doc))
	if err != nil {
		panic(err)
	}
	fmt.Println("collections:", theme.Collections())

	obj, _ := theme.New("pill")
	obj.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 60, H: 20})

	before, _ := obj.State("bg")
	obj.Signal("select") // instant (in:0), also emits "selected"
	after, _ := obj.State("bg")
	fmt.Printf("bg: %s -> %s\n", before, after)
	fmt.Println("emitted:", <-obj.SignalsOut())

	r, _ := obj.PartRect("bg")
	fmt.Printf("bg rect: %dx%d\n", r.W, r.H)

	// Output:
	// collections: [pill]
	// bg: default -> active
	// emitted: selected
	// bg rect: 60x20
}
