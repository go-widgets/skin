// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package skin is a declarative theme + layout + interaction-state engine for
// the go-widgets toolkit — the "Edje analogue" of the fleet, named generically
// as skin.
//
// An app describes a widget's PARTS (rectangles, text, images), their per-STATE
// visuals (fill/ink/border/radius/geometry) and the SIGNAL→state PROGRAMS that
// wire interaction to appearance in a DATA FILE (`.skin.json`). Re-skinning —
// or building an entirely new control — then needs no Go recompile: the running
// program loads different bytes.
//
// The three layers of the toolkit compose cleanly:
//
//   - the existing Go layouts (VBox/HBox/Grid/Frame) do INTER-widget placement;
//   - skin does INTRA-widget parts + interaction (an [Object] is a single
//     [toolkit.Widget], so it drops straight into those layouts);
//   - [github.com/go-widgets/toolkit/anim] drives the transitions.
//
// # Format
//
// The canonical surface is JSON via the standard library's encoding/json, with
// ZERO third-party dependencies (a sovereignty requirement of the fleet). The
// parse pipeline is deliberately layered so a friendlier front-end (an HCL
// surface, say) can be bolted on LATER without touching the runtime:
//
//	raw bytes ──json──▶ document (documented Go struct model) ──validate──▶ *Theme
//
// See [Load]. Only the first arrow is JSON-specific; everything downstream of
// the document model is format-agnostic. We do NOT depend on hashicorp/hcl (we
// don't own it); an HCL front-end, if ever added, would emit the same document
// model and reuse the whole validator + runtime unchanged.
//
// # Size- and font-relative geometry
//
// Part geometry scales to ANY object size and font metrics. A rel1/rel2
// endpoint is a FRACTIONAL position across its reference box (relative, 0..1)
// plus a pixel offset and a FONT-RELATIVE offset (offset_em, in em =
// glyph-height units resolved against the active [github.com/go-widgets/toolkit]
// font). A part may additionally hold a width:height ratio inside its allotted
// rect via aspect/aspect_mode (the Edje aspect idea — none/neither/horizontal/
// vertical/both), seated by the part's align. So the shipped switch/check/card
// collections render correctly at arbitrary bounds and under any SetFont rather
// than assuming the default 5x7 face.
//
// # Runtime
//
// [Load] parses+validates every collection into a [Theme]. [Theme.New]
// instantiates a live [Object] for one collection; the Object is a full
// [toolkit.Widget]. Drive it with [Object.Signal] ("mouse,in", "mouse,down",
// …), advance transitions from the host frame loop with [Object.Tick], and
// paint with [Object.Draw]. Emitted signals leave via [Object.SignalsOut] so a
// host can wire them to an [github.com/go-widgets/mvvm.Command].
package skin
