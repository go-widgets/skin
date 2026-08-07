# go-widgets/skin

[![CI](https://github.com/go-widgets/skin/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/skin/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/go-widgets/skin?display_name=tag&sort=semver&color=0d9488)](https://github.com/go-widgets/skin/releases)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-skin-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/skin)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

A declarative **theme + layout + interaction-state engine** for the
[go-widgets/toolkit](https://github.com/go-widgets/toolkit) — the fleet's
*Edje analogue*, named generically as `skin`.

You describe a widget's **parts**, their **per-state visuals**, and the
**signal → state transitions** in a data file (`.skin.json`). Re-skinning — or
building an entirely new control — then needs **no Go recompile**: the running
program loads different bytes.

Pure Go, standard library only (`encoding/json`), CGO-free, BSD-3-Clause,
**100% statement coverage**, validated on all six 64-bit Go targets.

## Why

The toolkit's widgets are hand-coded in Go: a `Button`'s parts, its hover /
pressed / disabled faces and its click feedback all live in `button.go`. That
is fast and precise, but every visual tweak is a code change and a rebuild, and
every new control is a new Go type.

`skin` moves that description into data. One JSON document can express a button,
a switch, a chip — their parts, their states, and how signals move them between
states — and the same engine renders and animates all of them. The three layers
compose cleanly:

| layer | responsibility |
|-------|----------------|
| the toolkit's `VBox`/`HBox`/`Grid`/`Frame` | **inter**-widget placement |
| **`skin`** | **intra**-widget parts + interaction-state |
| [`toolkit/anim`](https://github.com/go-widgets/toolkit/tree/main/anim) | driving the transitions |

A `skin.Object` **is** a `toolkit.Widget`, so it drops straight into the
existing layouts next to hand-coded widgets.

## Quickstart

```go
theme, err := skin.Load(jsonBytes)          // parse + validate every collection
if err != nil { log.Fatal(err) }

obj, _ := theme.New("button")               // one live instance
obj.Bind(skin.NewMVVMContext().Set("$.label", labelObs))  // data binding
obj.SetBounds(toolkit.Rect{W: 96, H: 28})

// In your frame loop:
obj.OnEvent(ev)                              // pointer events → signals → states
busy := obj.Tick(time.Now())                // advance transitions (anim driver)
obj.Draw(painter, theme.Palette())           // paint; satisfies toolkit.Widget

// React to a program's `emit`:
select {
case sig := <-obj.SignalsOut(): cmd.Execute() // e.g. wire to an mvvm.Command
default:
}
```

## Format

The canonical surface is **JSON**, decoded with the standard library — **zero
third-party dependencies** (a sovereignty requirement of the fleet). The parse
pipeline is deliberately layered:

```
raw bytes ──encoding/json──▶ document (Go struct model) ──validate──▶ *skin.Theme
```

Only the first arrow is JSON-specific. **JSON now; HCL later:** a friendlier
front-end (an HCL surface, say) can be added later by emitting the same document
model and reusing the whole validator + runtime unchanged. We deliberately do
**not** depend on `hashicorp/hcl` — we don't own it, and the fleet has been
removing third-party dependencies, not adding them; an owned HCL surface can be
added when it's wanted.

### Schema (`.skin.json`)

```jsonc
{
  "collections": {
    "button": {                       // a named widget/layout template
      "min": { "w": 96, "h": 28 },
      "parts": [
        {
          "name": "bg",
          "type": "rect",             // rect | text | image
          "rel1": { "to": "", "relative": [0,0], "offset": [0,0] },
          "rel2": { "to": "", "relative": [1,1], "offset": [0,0] },
          "states": {                 // default | hover | pressed | disabled | custom…
            "default":  { "color": "@surface",     "border": "@border",   "radius": 6 },
            "hover":    { "color": "@surface_alt", "border": "@border",   "radius": 6 },
            "pressed":  { "color": "@accent",      "border": "@border",   "radius": 6 },
            "disabled": { "color": "@muted_face",  "border": "@muted_ink","radius": 6 }
          }
        },
        {
          "name": "label", "type": "text", "text_from": "$.label",
          "align": [0.5, 0.5],
          "states": { "default": { "ink": "@on_surface" }, "pressed": { "ink": "@background" } }
        }
      ],
      "programs": [
        { "on": "mouse,in",  "target": ["bg","label"], "to": "hover",   "in": 0.12, "ease": "ease_out_cubic" },
        { "on": "mouse,down","target": ["bg","label"], "to": "pressed", "in": 0.06 },
        { "on": "mouse,click", "emit": "clicked" }
      ]
    }
  }
}
```

- **`part`** — `name`, `type` (`rect`/`text`/`image`), default geometry
  (`rel1`/`rel2`/`align`), an optional `text_from` binding path (or static
  `text`) or `image` drawable name, and a map of `states`.
- **`rel1`/`rel2`** — geometry endpoints. Each is a fraction (`relative`) of a
  reference box plus a pixel `offset`. The reference is the object itself
  (`to: ""`) or an **earlier** sibling part (`to: "<name>"`). `rel1` is the
  top-left, `rel2` the far corner. A `state` may override the endpoints, so a
  part can **move or resize between states** — that is how a switch knob slides.
- **`state`** — visual props: `color` (fill), `ink` (text/glyph), `border`,
  `border_width`, `radius` (a number or `"pill"`), `visible`, and optional
  per-state geometry (`rel1`/`rel2`/`align`). Colour values are `@tokens`
  (`@surface`, `@accent`, `@muted_ink`, `@accent_fg`, `@extra:<gtk-color>`),
  `#RRGGBB(AA)` hex, or `[r,g,b(,a)]` arrays.
- **`program`** — `on` (a signal), `target` (part names), `to` (their new
  state), `in` (transition seconds, driven by `toolkit/anim`), `ease` (a named
  toolkit easing) and an optional outward `emit`.

`@tokens` resolve against a `toolkit.Theme` — a **superset of**
[`LoadGTKTheme`](https://github.com/go-widgets/toolkit): every canonical palette
field plus every GTK `@define-color` entry (via `@extra:<name>`).

## Composition with the toolkit

Because `Object` implements the full `toolkit.Widget` interface
(`Bounds`/`SetBounds`/`Draw`/`HitTest`/`OnEvent`), a skinned control is placed
by the existing Go layouts exactly like any hand-coded widget:

```go
row := toolkit.NewHBox()
row.Add(skinnedButton)   // a *skin.Object
row.Add(toolkit.NewLabel("hand-coded neighbour"))
```

`skin` owns what is *inside* a widget (its parts and their interaction states);
the toolkit's layouts own where widgets *sit*.

## Parity

The engine is proven against reality: the toolkit's hand-coded **button**,
**check**, **switch**, **chip** and **card** are each re-expressed as a
`.skin.json` collection ([`testdata/`](./testdata)) and asserted **pixel-for-pixel
identical** to the Go originals across their `default`/`hover`/`pressed`/`disabled`
states (`parity_test.go`). A skinned `Draw` measures within **~1.5×** of the
hand-coded `Draw` (`bench_test.go`; observed ≈1.0×).

## License

BSD-3-Clause — see [LICENSE](./LICENSE). Copyright (c) 2026 the go-widgets/skin
authors.
