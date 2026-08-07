// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import "github.com/go-widgets/mvvm"

// MVVMContext is a [TextSource] backed by [github.com/go-widgets/mvvm]
// observables: it maps a document's text_from paths (e.g. "$.title") to live
// string [mvvm.Observable]s. Because [Object.Draw] pulls the current value each
// frame, an observable's later Set shows on the next paint — the data binding
// the toolkit's MVVM layer is meant to provide, wired into skin's declarative
// text parts.
type MVVMContext struct {
	obs map[string]*mvvm.Observable[string]
}

// NewMVVMContext returns an empty context. Register paths with [MVVMContext.Set].
func NewMVVMContext() *MVVMContext {
	return &MVVMContext{obs: map[string]*mvvm.Observable[string]{}}
}

// Set binds a text_from path to an observable and returns the context so calls
// chain: ctx.Set("$.title", title).Set("$.subtitle", subtitle).
func (c *MVVMContext) Set(path string, o *mvvm.Observable[string]) *MVVMContext {
	c.obs[path] = o
	return c
}

// Get resolves path to its observable's current value, reporting ok=false when
// the path is unbound so the part falls back to its static Text.
func (c *MVVMContext) Get(path string) (string, bool) {
	o, ok := c.obs[path]
	if !ok {
		return "", false
	}
	return o.Get(), true
}
