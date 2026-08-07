// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/skin"
	"github.com/go-widgets/toolkit"
)

// renderObj paints an object into a fresh cw×ch canvas and returns the bytes.
func renderObj(o *skin.Object, cw, ch int, th *toolkit.Theme) []byte {
	buf := make([]byte, cw*ch*4)
	o.Draw(painter.NewPixelPainter(buf, cw, ch), th)
	return buf
}

// loadTheme parses a testdata file into a *skin.Theme.
func loadTheme(t *testing.T, file string) *skin.Theme {
	t.Helper()
	src, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	th, err := skin.Load(src)
	if err != nil {
		t.Fatalf("load %s: %v", file, err)
	}
	return th
}

func mustLoad(t *testing.T, src string) *skin.Theme {
	t.Helper()
	th, err := skin.Load([]byte(src))
	if err != nil {
		t.Fatalf("load: %v\nsrc=%s", err, src)
	}
	return th
}

func TestNewUnknownCollection(t *testing.T) {
	th := loadTheme(t, "button.skin.json")
	_, err := th.New("nope")
	if err == nil {
		t.Fatal("expected error")
	}
	var uce *skin.UnknownCollectionError
	if !errors.As(err, &uce) || uce.Name != "nope" {
		t.Fatalf("want UnknownCollectionError{nope}, got %v", err)
	}
	if got := err.Error(); got != "skin: no collection named nope" {
		t.Fatalf("message = %q", got)
	}
}

func TestStateAccessorsAndSetState(t *testing.T) {
	o := loadCollection(t, "button.skin.json", "button")
	if _, ok := o.State("ghost"); ok {
		t.Fatal("unknown part reported ok")
	}
	if s, ok := o.State("bg"); !ok || s != "default" {
		t.Fatalf("initial state = %q,%v", s, ok)
	}
	o.SetState("disabled")
	if s, _ := o.State("bg"); s != "disabled" {
		t.Fatalf("after SetState = %q", s)
	}
	if o.Animating() {
		t.Fatal("SetState should not animate")
	}
}

func TestTransitionReachesTargetOverFakeClock(t *testing.T) {
	th := toolkit.DefaultLight()
	const cw, ch = 96, 28
	def := loadCollection(t, "button.skin.json", "button")
	def.Bind(mapSource{"$.label": "OK"})
	def.SetState("default")
	defBuf := renderObj(def, cw, ch, th)

	hov := loadCollection(t, "button.skin.json", "button")
	hov.Bind(mapSource{"$.label": "OK"})
	hov.SetState("hover")
	hovBuf := renderObj(hov, cw, ch, th)

	o := loadCollection(t, "button.skin.json", "button")
	o.Bind(mapSource{"$.label": "OK"})
	o.Signal("mouse,in")
	if !o.Animating() {
		t.Fatal("expected animating after mouse,in")
	}
	base := time.Unix(0, 0)
	o.Tick(base) // progress 0
	if busy := o.Tick(base.Add(60 * time.Millisecond)); !busy {
		t.Fatal("expected busy mid-transition")
	}
	midBuf := renderObj(o, cw, ch, th)
	if bytes.Equal(midBuf, defBuf) || bytes.Equal(midBuf, hovBuf) {
		t.Fatal("mid-transition frame should differ from both endpoints")
	}
	if busy := o.Tick(base.Add(200 * time.Millisecond)); busy {
		t.Fatal("transition should be finished (not busy)")
	}
	if o.Animating() {
		t.Fatal("still animating after end")
	}
	if s, _ := o.State("bg"); s != "hover" {
		t.Fatalf("settled state = %q, want hover", s)
	}
	endBuf := renderObj(o, cw, ch, th)
	if !bytes.Equal(endBuf, hovBuf) {
		t.Fatal("finished transition must match the hover target exactly")
	}
}

func TestTransitionInterruptAndInstant(t *testing.T) {
	o := loadCollection(t, "switch.skin.json", "switch")
	// Instant path already-settled: Signal to the state it's already in.
	o.Signal("toggle,off") // to "default" while already default → no-op
	if o.Animating() {
		t.Fatal("no-op signal should not animate")
	}
	// Start a transition, then interrupt it (cancels the running handle).
	o.Signal("toggle,on")
	o.Tick(time.Unix(0, 0))
	if !o.Animating() {
		t.Fatal("expected animating")
	}
	o.Signal("toggle,off") // interrupt: cancel + restart toward default
	// SetState mid-flight cancels the handle too.
	o.SetState("default")
	if o.Animating() {
		t.Fatal("SetState should cancel the transition")
	}
}

func TestInstantTransitionZeroDuration(t *testing.T) {
	o := loadCollection(t, "check.skin.json", "check")
	o.Signal("toggle,on") // in:0 → snaps instantly
	if o.Animating() {
		t.Fatal("in:0 must not animate")
	}
	if s, _ := o.State("box"); s != "checked" {
		t.Fatalf("box state = %q", s)
	}
}

func TestSignalsOutEmitAndOverflow(t *testing.T) {
	th := mustLoad(t, `{"collections":{"c":{"parts":[{"name":"p","type":"rect","states":{"default":{"color":"@surface"}}}],"programs":[{"on":"ping","emit":"pong"}]}}}`)
	o, err := th.New("c")
	if err != nil {
		t.Fatal(err)
	}
	const n = 25 // exceeds the 16-slot channel so the pending buffer is used
	for i := 0; i < n; i++ {
		o.Signal("ping")
	}
	got := 0
	deadline := time.Unix(0, 0)
	// Each round: drain whatever is on the channel, then Tick to flush any
	// still-pending emits onto it. Order is preserved and nothing is lost.
	for round := 0; round < n+5 && got < n; round++ {
		for draining := true; draining; {
			select {
			case s := <-o.SignalsOut():
				if s != "pong" {
					t.Fatalf("emit = %q", s)
				}
				got++
			default:
				draining = false
			}
		}
		deadline = deadline.Add(time.Millisecond)
		o.Tick(deadline)
	}
	if got != n {
		t.Fatalf("received %d emits, want %d", got, n)
	}
}

func TestBindMVVMAndFallback(t *testing.T) {
	th := toolkit.DefaultLight()
	title := mvvm.NewObservable("OK")
	ctx := skin.NewMVVMContext().Set("$.label", title)
	o := loadCollection(t, "button.skin.json", "button")

	// Unbound: label part (text_from only, no static text) draws nothing.
	blank := renderObj(o, 96, 28, th)
	o.Bind(ctx)
	bound := renderObj(o, 96, 28, th)
	if bytes.Equal(blank, bound) {
		t.Fatal("binding a label should change the render")
	}
	// Live update flows on next paint.
	title.Set("GO")
	updated := renderObj(o, 96, 28, th)
	if bytes.Equal(updated, bound) {
		t.Fatal("observable change should show on next paint")
	}
	// Clearing the binding falls back (to empty static text → blank again).
	o.Bind(nil)
	if !bytes.Equal(renderObj(o, 96, 28, th), blank) {
		t.Fatal("clearing binding should fall back to static text")
	}
}

func TestTextStaticFallback(t *testing.T) {
	th := toolkit.DefaultLight()
	src := `{"collections":{"c":{"min":{"w":60,"h":12},"parts":[{"name":"t","type":"text","text_from":"$.x","text":"fallback","rel1":{},"rel2":{"relative":[1,1]},"states":{"default":{"ink":"@on_surface"}}}]}}}`
	o, _ := mustLoad(t, src).New("c")
	o.Bind(mapSource{}) // no $.x → Get returns false → static "fallback"
	fb := renderObj(o, 60, 12, th)
	o.Bind(mapSource{"$.x": "live"})
	live := renderObj(o, 60, 12, th)
	if bytes.Equal(fb, live) {
		t.Fatal("fallback and live text should differ")
	}
	if bytes.Equal(fb, make([]byte, len(fb))) {
		t.Fatal("fallback text should draw pixels")
	}
}

func TestTextDefaultInk(t *testing.T) {
	th := toolkit.DefaultLight()
	// text part whose default state sets no ink → ink defaults to OnSurface.
	src := `{"collections":{"c":{"min":{"w":40,"h":12},"parts":[{"name":"t","type":"text","text":"hi","rel1":{},"rel2":{"relative":[1,1]},"states":{"default":{}}}]}}}`
	o, _ := mustLoad(t, src).New("c")
	buf := renderObj(o, 40, 12, th)
	if bytes.Equal(buf, make([]byte, len(buf))) {
		t.Fatal("text with default ink should still paint")
	}
}

func TestStateAlignOverride(t *testing.T) {
	th := toolkit.DefaultLight()
	// The default state overrides the part's align, exercising both the
	// validator's state-align path and paintText's st.align branch. Two
	// different aligns must place the text differently.
	left := `{"collections":{"c":{"min":{"w":60,"h":12},"parts":[{"name":"t","type":"text","text":"hi","align":[0.5,0.5],"rel1":{},"rel2":{"relative":[1,1]},"states":{"default":{"ink":"@on_surface","align":[0,0.5]}}}]}}}`
	center := `{"collections":{"c":{"min":{"w":60,"h":12},"parts":[{"name":"t","type":"text","text":"hi","rel1":{},"rel2":{"relative":[1,1]},"states":{"default":{"ink":"@on_surface","align":[1,0.5]}}}]}}}`
	lo, _ := mustLoad(t, left).New("c")
	co, _ := mustLoad(t, center).New("c")
	if bytes.Equal(renderObj(lo, 60, 12, th), renderObj(co, 60, 12, th)) {
		t.Fatal("different state-level aligns should render differently")
	}
}

func TestMVVMContextMiss(t *testing.T) {
	ctx := skin.NewMVVMContext()
	if _, ok := ctx.Get("$.absent"); ok {
		t.Fatal("unbound path should report ok=false")
	}
}

func TestImagePart(t *testing.T) {
	th := toolkit.DefaultLight()
	o := loadCollection(t, "demo.skin.json", "badge")
	// Unregistered drawable: image part is skipped (no panic).
	_ = renderObj(o, 60, 20, th)
	painted := 0
	o.SetImage("star", func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
		p.FillRect(r, ink)
		painted++
	})
	_ = renderObj(o, 60, 20, th)
	if painted == 0 {
		t.Fatal("registered drawable was not invoked")
	}
	o.SetImage("star", nil) // delete branch
	painted = 0
	_ = renderObj(o, 60, 20, th)
	if painted != 0 {
		t.Fatal("deleted drawable was still invoked")
	}
}

func TestImageDefaultInk(t *testing.T) {
	th := toolkit.DefaultLight()
	// image part whose default state sets no ink → ink defaults to OnSurface.
	src := `{"collections":{"c":{"min":{"w":20,"h":20},"parts":[{"name":"i","type":"image","image":"x","rel1":{},"rel2":{"relative":[1,1]},"states":{"default":{}}}]}}}`
	o, _ := mustLoad(t, src).New("c")
	var gotInk toolkit.RGBA
	o.SetImage("x", func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) { gotInk = ink })
	_ = renderObj(o, 20, 20, th)
	if gotInk != th.OnSurface {
		t.Fatalf("default ink = %v, want OnSurface %v", gotInk, th.OnSurface)
	}
}

func TestZeroSizePartIsSkipped(t *testing.T) {
	th := toolkit.DefaultLight()
	// rel1 == rel2 → zero-width/height rect; paintRect must guard and skip.
	src := `{"collections":{"c":{"min":{"w":10,"h":10},"parts":[{"name":"p","type":"rect","rel1":{"offset":[5,5]},"rel2":{"relative":[0,0],"offset":[5,5]},"states":{"default":{"color":"@surface","border":"@border"}}}]}}}`
	o, _ := mustLoad(t, src).New("c")
	buf := renderObj(o, 10, 10, th)
	if !bytes.Equal(buf, make([]byte, len(buf))) {
		t.Fatal("zero-size part should paint nothing")
	}
}

func TestBlendVisualBranches(t *testing.T) {
	th := toolkit.DefaultLight()
	// r: default has fill+border; state "a" has neither; state "b" has fill only.
	src := `{"collections":{"c":{"min":{"w":20,"h":20},"parts":[{"name":"r","type":"rect","rel1":{},"rel2":{"relative":[1,1]},"states":{` +
		`"default":{"color":"@surface","border":"@border"},` +
		`"a":{},` +
		`"b":{"color":"@accent"}}}],"programs":[` +
		`{"on":"toa","target":["r"],"to":"a","in":0.1},` +
		`{"on":"tob","target":["r"],"to":"b","in":0.1}]}}}`
	th2 := mustLoad(t, src)
	o, _ := th2.New("c")
	base := time.Unix(0, 0)
	// default→a: hasFrom-only (fill fades out, border fades out).
	o.Signal("toa")
	o.Tick(base)
	o.Tick(base.Add(50 * time.Millisecond))
	_ = renderObj(o, 20, 20, th) // exercises hasFrom-only + neither(after? still from)
	o.Tick(base.Add(200 * time.Millisecond))
	// a→b: hasTo-only (fill appears), border neither-side.
	o.Signal("tob")
	o.Tick(base.Add(300 * time.Millisecond))
	o.Tick(base.Add(350 * time.Millisecond))
	_ = renderObj(o, 20, 20, th)
	o.Tick(base.Add(600 * time.Millisecond))
	if s, _ := o.State("r"); s != "b" {
		t.Fatalf("state = %q", s)
	}
}

func TestOnEventDrivesSignals(t *testing.T) {
	o := loadCollection(t, "button.skin.json", "button")
	o.SetBounds(toolkit.Rect{W: 96, H: 28})
	o.OnEvent(toolkit.Event{Kind: toolkit.EventMouseMove, X: 5, Y: 5}) // inside → mouse,in
	if s, _ := o.State("bg"); s != "hover" {
		t.Fatalf("after move-in: %q", s)
	}
	o.OnEvent(toolkit.Event{Kind: toolkit.EventMouseMove, X: -1, Y: 5}) // outside → mouse,out
	if s, _ := o.State("bg"); s != "default" {
		t.Fatalf("after move-out: %q", s)
	}
	o.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 5, Y: 5}) // down + click(emit)
	if s, _ := o.State("bg"); s != "pressed" {
		t.Fatalf("after click: %q", s)
	}
	select {
	case s := <-o.SignalsOut():
		if s != "clicked" {
			t.Fatalf("emit = %q", s)
		}
	default:
		t.Fatal("expected clicked emit")
	}
	o.OnEvent(toolkit.Event{Kind: toolkit.EventMouseUp, X: 5, Y: 5}) // up → hover
	if s, _ := o.State("bg"); s != "hover" {
		t.Fatalf("after up: %q", s)
	}
	o.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: "a"}) // unhandled kind: no-op
}

func TestPaletteAndSetPalette(t *testing.T) {
	th := loadTheme(t, "chip.skin.json")
	if th.Palette().Surface != toolkit.DefaultLight().Surface {
		t.Fatal("default palette should be DefaultLight")
	}
	custom := toolkit.DefaultDark()
	th.SetPalette(custom)
	if th.Palette() != custom {
		t.Fatal("SetPalette not applied")
	}
	th.SetPalette(nil) // nil restores DefaultLight
	if th.Palette().Surface != toolkit.DefaultLight().Surface {
		t.Fatal("nil SetPalette should restore DefaultLight")
	}
}
