// Copyright (c) 2026 the go-widgets/skin authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package skin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-widgets/toolkit"
)

// ---- raw document model (the documented struct model JSON decodes into) ----
//
// This layer is the ONLY JSON-specific surface. A future HCL front-end would
// build the same document + hand it to validate, reusing everything below.

type document struct {
	Collections map[string]*rawCollection `json:"collections"`
}

type rawCollection struct {
	Min      *rawSize      `json:"min"`
	Parts    []*rawPart    `json:"parts"`
	Programs []*rawProgram `json:"programs"`
}

type rawSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type rawPart struct {
	Name     string               `json:"name"`
	Type     string               `json:"type"`
	Rel1     *rawRel              `json:"rel1"`
	Rel2     *rawRel              `json:"rel2"`
	Align    *[2]float64          `json:"align"`
	TextFrom string               `json:"text_from"`
	Text     string               `json:"text"`
	Image    string               `json:"image"`
	States   map[string]*rawState `json:"states"`
}

type rawRel struct {
	To       string      `json:"to"`
	Relative *[2]float64 `json:"relative"`
	Offset   *[2]int     `json:"offset"`
}

type rawState struct {
	Color       json.RawMessage `json:"color"`
	Ink         json.RawMessage `json:"ink"`
	Border      json.RawMessage `json:"border"`
	BorderWidth *int            `json:"border_width"`
	Radius      json.RawMessage `json:"radius"`
	Visible     *bool           `json:"visible"`
	Rel1        *rawRel         `json:"rel1"`
	Rel2        *rawRel         `json:"rel2"`
	Align       *[2]float64     `json:"align"`
}

type rawProgram struct {
	On     string   `json:"on"`
	Target []string `json:"target"`
	To     string   `json:"to"`
	In     float64  `json:"in"`
	Ease   string   `json:"ease"`
	Emit   string   `json:"emit"`
}

// easings maps the document's snake_case ease names to the toolkit's curves.
// An empty name defaults to Linear at the call site.
var easings = map[string]toolkit.Easing{
	"linear":            toolkit.Linear,
	"ease_in_quad":      toolkit.EaseInQuad,
	"ease_out_quad":     toolkit.EaseOutQuad,
	"ease_in_out_quad":  toolkit.EaseInOutQuad,
	"ease_in_cubic":     toolkit.EaseInCubic,
	"ease_out_cubic":    toolkit.EaseOutCubic,
	"ease_in_out_cubic": toolkit.EaseInOutCubic,
}

// partTypes maps the document's type strings to PartType.
var partTypes = map[string]PartType{
	"rect": PartRect, "text": PartText, "image": PartImage,
}

// Load parses jsonSrc and validates every collection, returning a ready [Theme]
// whose palette defaults to [toolkit.DefaultLight]. Unknown JSON keys are
// rejected (a typo'd field is an error, not a silent no-op). Every failure is a
// rich error naming the offending collection/part/state/program.
func Load(jsonSrc []byte) (*Theme, error) {
	var doc document
	dec := json.NewDecoder(bytes.NewReader(jsonSrc))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("skin: parse JSON: %w", err)
	}
	return validate(&doc)
}

// validate turns a decoded document into a *Theme, or a rich error.
func validate(doc *document) (*Theme, error) {
	if len(doc.Collections) == 0 {
		return nil, fmt.Errorf("skin: document defines no collections")
	}
	t := &Theme{
		collections: make(map[string]*Collection, len(doc.Collections)),
		palette:     toolkit.DefaultLight(),
	}
	for _, name := range sortedNames(doc.Collections) {
		c, err := validateCollection(name, doc.Collections[name])
		if err != nil {
			return nil, err
		}
		t.collections[name] = c
		t.names = append(t.names, name)
	}
	return t, nil
}

// validateCollection validates one collection (its size, parts, then programs).
func validateCollection(name string, rc *rawCollection) (*Collection, error) {
	if rc == nil {
		return nil, fmt.Errorf("skin: collection %q is null", name)
	}
	c := &Collection{Name: name, partIndex: map[string]int{}}
	if rc.Min != nil {
		if rc.Min.W < 0 || rc.Min.H < 0 {
			return nil, fmt.Errorf("skin: collection %q: min size cannot be negative", name)
		}
		c.MinW, c.MinH = rc.Min.W, rc.Min.H
	}
	if len(rc.Parts) == 0 {
		return nil, fmt.Errorf("skin: collection %q has no parts", name)
	}
	for i, rp := range rc.Parts {
		p, err := validatePart(name, i, rp, c.partIndex)
		if err != nil {
			return nil, err
		}
		c.partIndex[p.Name] = len(c.Parts)
		c.Parts = append(c.Parts, p)
	}
	for i, rpr := range rc.Programs {
		pr, err := validateProgram(name, i, rpr, c.partIndex, c.Parts)
		if err != nil {
			return nil, err
		}
		c.Programs = append(c.Programs, pr)
	}
	return c, nil
}

// validatePart validates one part. seen holds the names of parts already
// declared in this collection, so a rel `to` can be checked against "earlier
// siblings only" (no forward references, no self-reference, no cycles).
func validatePart(coll string, idx int, rp *rawPart, seen map[string]int) (*Part, error) {
	where := fmt.Sprintf("skin: collection %q part #%d", coll, idx)
	if rp == nil {
		return nil, fmt.Errorf("%s is null", where)
	}
	if rp.Name == "" {
		return nil, fmt.Errorf("%s has an empty name", where)
	}
	where = fmt.Sprintf("skin: collection %q part %q", coll, rp.Name)
	if _, dup := seen[rp.Name]; dup {
		return nil, fmt.Errorf("%s: duplicate part name", where)
	}
	pt, ok := partTypes[rp.Type]
	if !ok {
		return nil, fmt.Errorf("%s: unknown type %q (rect|text|image)", where, rp.Type)
	}
	p := &Part{
		Name: rp.Name, Type: pt,
		TextFrom: rp.TextFrom, Text: rp.Text, Image: rp.Image,
		states: map[string]*State{},
	}
	if rp.Align != nil {
		p.align = *rp.Align
	} else {
		p.align = [2]float64{0, 0}
	}
	r1, err := convRel(where+": rel1", rp.Rel1, false, seen)
	if err != nil {
		return nil, err
	}
	r2, err := convRel(where+": rel2", rp.Rel2, true, seen)
	if err != nil {
		return nil, err
	}
	p.rel1, p.rel2 = r1, r2
	if len(rp.States) == 0 {
		return nil, fmt.Errorf("%s: no states", where)
	}
	if _, ok := rp.States["default"]; !ok {
		return nil, fmt.Errorf("%s: missing required \"default\" state", where)
	}
	for _, sn := range sortedNames(rp.States) {
		st, err := validateState(where, sn, rp.States[sn], seen)
		if err != nil {
			return nil, err
		}
		p.states[sn] = st
	}
	return p, nil
}

// validateState validates one part description (the visual for a state).
func validateState(partWhere, name string, rs *rawState, seen map[string]int) (*State, error) {
	where := fmt.Sprintf("%s state %q", partWhere, name)
	if rs == nil {
		return nil, fmt.Errorf("%s is null", where)
	}
	st := &State{Name: name, borderW: 1, visible: true}
	if rs.Visible != nil {
		st.visible = *rs.Visible
	}
	var err error
	if rs.Color != nil {
		if st.fill, err = parseColor(where+": color", rs.Color); err != nil {
			return nil, err
		}
		st.hasFill = true
	}
	if rs.Ink != nil {
		if st.ink, err = parseColor(where+": ink", rs.Ink); err != nil {
			return nil, err
		}
		st.hasInk = true
	}
	if rs.Border != nil {
		if st.border, err = parseColor(where+": border", rs.Border); err != nil {
			return nil, err
		}
		st.hasBorder = true
	}
	if rs.BorderWidth != nil {
		if *rs.BorderWidth < 0 {
			return nil, fmt.Errorf("%s: border_width cannot be negative", where)
		}
		st.borderW = *rs.BorderWidth
	}
	if st.radius, err = parseRadius(where+": radius", rs.Radius); err != nil {
		return nil, err
	}
	if rs.Rel1 != nil {
		r, err := convRel(where+": rel1", rs.Rel1, false, seen)
		if err != nil {
			return nil, err
		}
		st.rel1 = &r
	}
	if rs.Rel2 != nil {
		r, err := convRel(where+": rel2", rs.Rel2, true, seen)
		if err != nil {
			return nil, err
		}
		st.rel2 = &r
	}
	if rs.Align != nil {
		a := *rs.Align
		st.align = &a
	}
	return st, nil
}

// parseRadius parses the polymorphic radius value (absent → 0, a number, or the
// string "pill").
func parseRadius(where string, raw json.RawMessage) (radiusSpec, error) {
	if raw == nil {
		return radiusSpec{}, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == `"pill"` {
		return radiusSpec{pill: true}, nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return radiusSpec{}, fmt.Errorf("%s: only \"pill\" or a number is allowed, got %s", where, trimmed)
	}
	var px int
	if err := json.Unmarshal(raw, &px); err != nil {
		return radiusSpec{}, fmt.Errorf("%s: %w", where, err)
	}
	if px < 0 {
		return radiusSpec{}, fmt.Errorf("%s: radius cannot be negative", where)
	}
	return radiusSpec{px: px}, nil
}

// convRel converts a raw endpoint into a relSpec, applying the per-endpoint
// defaults (rel1 anchors at the box origin (0,0); rel2 at the far corner
// (1,1)) and checking that a non-empty `to` names an EARLIER sibling.
func convRel(where string, rr *rawRel, isRel2 bool, seen map[string]int) (relSpec, error) {
	rs := relSpec{}
	if isRel2 {
		rs.relx, rs.rely = 1, 1
	}
	if rr == nil {
		return rs, nil
	}
	rs.to = rr.To
	if rr.Relative != nil {
		rs.relx, rs.rely = rr.Relative[0], rr.Relative[1]
	}
	if rr.Offset != nil {
		rs.offx, rs.offy = rr.Offset[0], rr.Offset[1]
	}
	if rs.to != "" {
		if _, ok := seen[rs.to]; !ok {
			return relSpec{}, fmt.Errorf("%s: `to` references %q which is not an earlier part in this collection", where, rs.to)
		}
	}
	return rs, nil
}

// validateProgram validates one program against the collection's parts.
func validateProgram(coll string, idx int, rpr *rawProgram, index map[string]int, parts []*Part) (*Program, error) {
	where := fmt.Sprintf("skin: collection %q program #%d", coll, idx)
	if rpr == nil {
		return nil, fmt.Errorf("%s is null", where)
	}
	if rpr.On == "" {
		return nil, fmt.Errorf("%s: empty `on` signal", where)
	}
	if rpr.In < 0 {
		return nil, fmt.Errorf("%s: `in` (transition seconds) cannot be negative", where)
	}
	pr := &Program{On: rpr.On, To: rpr.To, In: rpr.In, Emit: rpr.Emit, Ease: toolkit.Linear}
	if rpr.Ease != "" {
		e, ok := easings[rpr.Ease]
		if !ok {
			return nil, fmt.Errorf("%s: unknown ease %q", where, rpr.Ease)
		}
		pr.Ease = e
	}
	// A program with a target must name a state; every target part must both
	// exist and describe that state.
	if len(rpr.Target) > 0 && rpr.To == "" {
		return nil, fmt.Errorf("%s: has targets but no `to` state", where)
	}
	for _, tn := range rpr.Target {
		pi, ok := index[tn]
		if !ok {
			return nil, fmt.Errorf("%s: target %q is not a part in this collection", where, tn)
		}
		if _, ok := parts[pi].states[rpr.To]; !ok {
			return nil, fmt.Errorf("%s: target %q has no state %q", where, tn, rpr.To)
		}
		pr.Target = append(pr.Target, tn)
	}
	if len(pr.Target) == 0 && pr.Emit == "" {
		return nil, fmt.Errorf("%s: program does nothing (no targets and no emit)", where)
	}
	return pr, nil
}
