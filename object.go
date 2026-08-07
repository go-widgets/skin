// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"time"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/toolkit/anim"
)

// signalBuf is the capacity of an Object's outward-signal channel. Emitted
// signals that don't fit are buffered internally and flushed the moment the
// host drains the channel, so none are lost and no emit ever blocks.
const signalBuf = 16

// Drawable is a host-registered painter for a PartImage — the "swallow" seam
// for content the rect/text vocabulary can't express (a vector icon, a photo).
// It is handed the part's resolved rect and the part's current ink so the glyph
// can track state tints (pressed/disabled) exactly like a toolkit widget's
// host-supplied Icon callback.
type Drawable func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA)

// TextSource resolves a part's text_from path (e.g. "$.title") to a live
// string. Get reports ok=false when the path is unbound, so the part falls back
// to its static Text. See [MVVMContext] for an [github.com/go-widgets/mvvm]
// -backed implementation.
type TextSource interface {
	Get(path string) (string, bool)
}

// Object is one live instance of a collection: a full [toolkit.Widget] that
// owns its parts' current states and any in-flight transitions. Drop it into a
// VBox/HBox/Grid like any other widget; drive its state machine with
// [Object.Signal] and advance transitions with [Object.Tick].
type Object struct {
	toolkit.Base

	coll   *Collection
	th     *Theme
	parts  []*partState
	byName map[string]*partState

	driver *anim.Driver
	src    TextSource
	images map[string]Drawable

	out     chan string
	pending []string
}

// partState is an Object's per-part runtime: the settled state it is in (or
// heading to), and, while a transition runs, the state it is animating from and
// how far along (prog in [0,1]).
type partState struct {
	p       *Part
	state   string // the settled state, or the transition's target
	from    string // the state being animated away from
	prog    float64
	running bool
	handle  *anim.Handle
}

// New instantiates the named collection as a live Object, sized to the
// collection's min. It errors if the collection is not in this Theme.
func (t *Theme) New(collection string) (*Object, error) {
	c, ok := t.collections[collection]
	if !ok {
		return nil, &UnknownCollectionError{Name: collection}
	}
	o := &Object{
		coll:   c,
		th:     t,
		byName: make(map[string]*partState, len(c.Parts)),
		driver: anim.NewDriver(),
		images: map[string]Drawable{},
		out:    make(chan string, signalBuf),
	}
	for _, p := range c.Parts {
		ps := &partState{p: p, state: "default", from: "default", prog: 1}
		o.parts = append(o.parts, ps)
		o.byName[p.Name] = ps
	}
	o.SetBounds(toolkit.Rect{W: c.MinW, H: c.MinH})
	return o, nil
}

// UnknownCollectionError reports that [Theme.New] was asked for a collection
// the document doesn't define.
type UnknownCollectionError struct{ Name string }

func (e *UnknownCollectionError) Error() string {
	return "skin: no collection named " + e.Name
}

// Bind sets the source Object resolves text_from paths against. Binding is a
// pull model: [Object.Draw] reads the current value each frame, so a later
// change to the source shows on the next paint with no re-bind. A nil src
// clears the binding (parts fall back to their static Text).
func (o *Object) Bind(src TextSource) { o.src = src }

// SetImage registers a [Drawable] under name so a PartImage referencing that
// name paints through it. Registering nil removes the drawable.
func (o *Object) SetImage(name string, d Drawable) {
	if d == nil {
		delete(o.images, name)
		return
	}
	o.images[name] = d
}

// SignalsOut returns the channel emitted signals are delivered on. Wire it to
// an [github.com/go-widgets/mvvm.Command] (or any handler) to react to a
// program's `emit`. The channel is buffered; drain it from the host loop.
func (o *Object) SignalsOut() <-chan string { return o.out }

// State returns the settled state a part is in (its transition TARGET while one
// runs). ok is false for an unknown part name. Handy for tests and for a host
// that mirrors part state into its model.
func (o *Object) State(part string) (state string, ok bool) {
	ps, ok := o.byName[part]
	if !ok {
		return "", false
	}
	return ps.state, true
}

// PartRect returns a part's resolved rectangle in its current settled state,
// against the object's current bounds — the frame it would paint into this
// tick. ok is false for an unknown part name. Useful for a host that hit-tests
// or overlays a specific sub-part (a chip's close slot, a card's header).
func (o *Object) PartRect(name string) (toolkit.Rect, bool) {
	if _, ok := o.byName[name]; !ok {
		return toolkit.Rect{}, false
	}
	anchors := o.coll.anchorMap(o.Bounds(), func(pt *Part) *State {
		return pt.stateFor(o.byName[pt.Name].state)
	})
	return anchors[name], true
}

// Animating reports whether any part currently has a transition in flight.
func (o *Object) Animating() bool {
	for _, ps := range o.parts {
		if ps.running {
			return true
		}
	}
	return false
}

// SetState instantly puts every part into the named state (no animation),
// cancelling any transition in flight. It models app-owned state — a control
// the app marks "disabled", a tab it marks "selected" — that is set directly
// rather than derived from pointer signals. Parts that don't describe name fall
// back to their "default" description at draw time.
func (o *Object) SetState(name string) {
	for _, ps := range o.parts {
		if ps.handle != nil {
			ps.handle.Cancel()
			ps.handle = nil
		}
		ps.from = name
		ps.state = name
		ps.prog = 1
		ps.running = false
	}
}

// Signal drives the state machine: every program whose `on` equals name fires,
// transitioning its target parts and raising its `emit` (if any) outward.
// Unmatched signals are a harmless no-op.
func (o *Object) Signal(name string) {
	for _, pr := range o.coll.Programs {
		if pr.On != name {
			continue
		}
		for _, tn := range pr.Target {
			o.startTransition(o.byName[tn], pr.To, pr.In, pr.Ease)
		}
		if pr.Emit != "" {
			o.pending = append(o.pending, pr.Emit)
		}
	}
	o.flush()
}

// startTransition moves ps toward state over `in` seconds shaped by ease. A
// non-positive `in` snaps instantly (no animation); an already-settled part
// asked for its current state does nothing.
func (o *Object) startTransition(ps *partState, state string, in float64, ease toolkit.Easing) {
	if ps.handle != nil {
		ps.handle.Cancel()
		ps.handle = nil
	}
	if in <= 0 {
		ps.from = state
		ps.state = state
		ps.prog = 1
		ps.running = false
		return
	}
	if !ps.running && ps.state == state {
		return
	}
	ps.from = ps.state
	ps.state = state
	ps.prog = 0
	ps.running = true
	ps.handle = o.driver.Start(&anim.Animation{
		Dur:   time.Duration(in * float64(time.Second)),
		Ease:  ease,
		Apply: func(p float64) { ps.prog = p },
		OnEnd: func() { ps.running = false; ps.prog = 1; ps.handle = nil },
	})
}

// Tick advances every in-flight transition to now via the toolkit anim driver
// and reports the driver's busy flag straight through: when it is false nothing
// is animating and the host may stop scheduling frames.
func (o *Object) Tick(now time.Time) bool {
	busy := o.driver.Tick(now)
	o.flush()
	return busy
}

// flush moves as many buffered emits as fit onto the outward channel without
// blocking; the rest wait for the next opportunity (the host draining the
// channel), preserving order and losing nothing.
func (o *Object) flush() {
	for len(o.pending) > 0 {
		select {
		case o.out <- o.pending[0]:
			o.pending = o.pending[1:]
		default:
			return
		}
	}
}

// OnEvent translates toolkit input into skin signals so an Object behaves like
// any interactive widget inside a container: hover in/out from EventMouseMove,
// press from EventClick (also raising the activation signal "mouse,click"), and
// release from EventMouseUp. A collection wires these to states via programs;
// events with no matching program are ignored.
func (o *Object) OnEvent(ev toolkit.Event) {
	switch ev.Kind {
	case toolkit.EventMouseMove:
		if o.localInside(ev.X, ev.Y) {
			o.Signal("mouse,in")
		} else {
			o.Signal("mouse,out")
		}
	case toolkit.EventClick:
		o.Signal("mouse,down")
		o.Signal("mouse,click")
	case toolkit.EventMouseUp:
		o.Signal("mouse,up")
	}
}

// localInside reports whether a widget-local point falls within the object.
func (o *Object) localInside(x, y int) bool {
	b := o.Bounds()
	return x >= 0 && y >= 0 && x < b.W && y < b.H
}

// stateFor returns the description a part uses for state name, falling back to
// its "default" description when it doesn't define name.
func (p *Part) stateFor(name string) *State {
	if st, ok := p.states[name]; ok {
		return st
	}
	return p.states["default"]
}

// Draw paints every part back-to-front through p using th's palette (pass
// [Theme.Palette] for the document's own palette). It satisfies
// [toolkit.Widget].
func (o *Object) Draw(p painter.Painter, th *toolkit.Theme) {
	obj := o.Bounds()
	// Anchor frame: place every part at its settled/target geometry so a part
	// that anchors to a sibling sees a stable rect even while that sibling (or
	// itself) is mid-transition.
	anchors := o.coll.anchorMap(obj, func(pt *Part) *State {
		return pt.stateFor(o.byName[pt.Name].state)
	})
	for _, ps := range o.parts {
		o.drawPart(p, th, ps, obj, anchors)
	}
}

// drawPart paints one part, blending its from/to descriptions while a
// transition runs and using the single settled description otherwise.
func (o *Object) drawPart(p painter.Painter, th *toolkit.Theme, ps *partState, obj toolkit.Rect, anchors map[string]toolkit.Rect) {
	toSt := ps.p.stateFor(ps.state)
	if !toSt.visible {
		return
	}
	rect := resolveRect(ps.p, toSt, obj, anchors)
	radius := toSt.radius.resolve(rect.W, rect.H)
	fill, hasFill := resolveVisual(toSt.hasFill, toSt.fill, th)
	ink, hasInk := resolveVisual(toSt.hasInk, toSt.ink, th)
	border, hasBorder := resolveVisual(toSt.hasBorder, toSt.border, th)

	if ps.running {
		fromSt := ps.p.stateFor(ps.from)
		fromRect := resolveRect(ps.p, fromSt, obj, anchors)
		rect = lerpRect(fromRect, rect, ps.prog)
		fr := fromSt.radius.resolve(fromRect.W, fromRect.H)
		radius = int(float64(fr) + (float64(radius)-float64(fr))*ps.prog)
		fill, hasFill = blendVisual(fromSt.hasFill, fromSt.fill, toSt.hasFill, toSt.fill, th, ps.prog)
		ink, hasInk = blendVisual(fromSt.hasInk, fromSt.ink, toSt.hasInk, toSt.ink, th, ps.prog)
		border, hasBorder = blendVisual(fromSt.hasBorder, fromSt.border, toSt.hasBorder, toSt.border, th, ps.prog)
	}

	switch ps.p.Type {
	case PartRect:
		paintRect(p, rect, hasFill, fill, hasBorder, border, toSt.borderW, radius)
	case PartText:
		o.paintText(p, ps.p, toSt, rect, ink, hasInk, th)
	default: // PartImage
		if d, ok := o.images[ps.p.Image]; ok {
			if !hasInk {
				ink = th.OnSurface
			}
			d(p, rect, ink)
		}
	}
}

// resolveVisual resolves a single (present?) colour to a concrete value.
func resolveVisual(has bool, cs colorSpec, th *toolkit.Theme) (toolkit.RGBA, bool) {
	if !has {
		return toolkit.RGBA{}, false
	}
	return cs.resolve(th), true
}

// blendVisual resolves a colour that is transitioning from one description to
// another. If both sides set it the result is the per-channel lerp; if only one
// side sets it that side's colour is used as-is (present throughout the tween).
func blendVisual(hasFrom bool, from colorSpec, hasTo bool, to colorSpec, th *toolkit.Theme, prog float64) (toolkit.RGBA, bool) {
	switch {
	case hasFrom && hasTo:
		return lerpColor(from.resolve(th), to.resolve(th), prog), true
	case hasTo:
		return to.resolve(th), true
	case hasFrom:
		return from.resolve(th), true
	default:
		return toolkit.RGBA{}, false
	}
}

// paintRect fills + strokes a rectangle exactly as the toolkit's internal
// fill/stroke shims do (same zero-size guard, same rounded vs square choice),
// so a skinned rect is byte-identical to a hand-coded one.
func paintRect(p painter.Painter, r toolkit.Rect, hasFill bool, fill toolkit.RGBA, hasBorder bool, border toolkit.RGBA, borderW, radius int) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if hasFill {
		if radius > 0 {
			p.FillRoundRect(r, radius, fill)
		} else {
			p.FillRect(r, fill)
		}
	}
	if hasBorder {
		if radius > 0 {
			p.StrokeRoundRect(r, radius, border, borderW)
		} else {
			p.StrokeRect(r, border, borderW)
		}
	}
}

// paintText positions and draws a part's text (binding value or static Text)
// with align, in the toolkit's active font — the same font + placement maths
// the hand-coded widgets use.
func (o *Object) paintText(p painter.Painter, part *Part, st *State, r toolkit.Rect, ink toolkit.RGBA, hasInk bool, th *toolkit.Theme) {
	text := part.Text
	if part.TextFrom != "" && o.src != nil {
		if v, ok := o.src.Get(part.TextFrom); ok {
			text = v
		}
	}
	if text == "" {
		return
	}
	if !hasInk {
		ink = th.OnSurface
	}
	align := part.align
	if st.align != nil {
		align = *st.align
	}
	tw := toolkit.TextWidth(text)
	gh := toolkit.GlyphHeight()
	tx := r.X + fracInt(align[0], r.W-tw)
	ty := r.Y + fracInt(align[1], r.H-gh)
	toolkit.DrawText(p, tx, ty, text, ink)
}
