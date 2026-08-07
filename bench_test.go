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

// BenchmarkDrawSkinButton and BenchmarkDrawToolkitButton report ns/Draw for a
// skinned button vs the hand-coded toolkit.Button. The parity gate proves they
// paint identically; these quantify the interpreter overhead. See
// TestDrawCostRatio for the enforced ceiling.

const benchW, benchH = 96, 28

func newBenchCanvas() (painter.Painter, func()) {
	buf := make([]byte, (benchW+12)*(benchH+12)*4)
	return painter.NewPixelPainter(buf, benchW+12, benchH+12), func() {}
}

func benchSkinButton(b testing.TB) *skin.Object {
	src, err := os.ReadFile("testdata/button.skin.json")
	if err != nil {
		b.Fatal(err)
	}
	th, err := skin.Load(src)
	if err != nil {
		b.Fatal(err)
	}
	o, err := th.New("button")
	if err != nil {
		b.Fatal(err)
	}
	o.Bind(mapSource{"$.label": "OK"})
	o.SetBounds(toolkit.Rect{X: 10, Y: 10, W: benchW, H: benchH})
	return o
}

func BenchmarkDrawSkinButton(b *testing.B) {
	th := toolkit.DefaultLight()
	o := benchSkinButton(b)
	p, done := newBenchCanvas()
	defer done()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Draw(p, th)
	}
}

func BenchmarkDrawToolkitButton(b *testing.B) {
	th := toolkit.DefaultLight()
	w := &toolkit.Button{Label: "OK", PressFeedback: true}
	w.SetBounds(toolkit.Rect{X: 10, Y: 10, W: benchW, H: benchH})
	p, done := newBenchCanvas()
	defer done()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Draw(p, th)
	}
}

// TestDrawCostRatio enforces the backlog's ceiling: a skinned Draw must stay
// within ~1.5× the hand-coded Draw. It measures both in-process (so it runs in
// the normal test pass and gates CI) and reports the ratio.
func TestDrawCostRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive ratio test in -short")
	}
	th := toolkit.DefaultLight()
	skinBtn := benchSkinButton(t)
	tkBtn := &toolkit.Button{Label: "OK", PressFeedback: true}
	tkBtn.SetBounds(toolkit.Rect{X: 10, Y: 10, W: benchW, H: benchH})
	p, done := newBenchCanvas()
	defer done()

	rs := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			skinBtn.Draw(p, th)
		}
	})
	rt := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tkBtn.Draw(p, th)
		}
	})
	skinNs := float64(rs.NsPerOp())
	tkNs := float64(rt.NsPerOp())
	ratio := skinNs / tkNs
	t.Logf("skin=%.0f ns/Draw  toolkit=%.0f ns/Draw  ratio=%.2fx", skinNs, tkNs, ratio)
	const ceiling = 1.5
	if ratio > ceiling {
		t.Fatalf("skin Draw is %.2fx the hand-coded Draw, exceeds %.1fx ceiling", ratio, ceiling)
	}
}
