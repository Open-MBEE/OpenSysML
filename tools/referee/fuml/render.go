package fuml

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// renderExpected spells the implementation's outputs one parameter per line,
// in the activity's parameter order.
func renderExpected(a *Activity, x *ExpectedActivity) string {
	byName := map[string]ExpectedOutput{}
	if x != nil {
		for _, o := range x.Outputs {
			byName[o.Parameter] = o
		}
	}
	g := newGraph(a.Model, nil)
	var lines []held
	for _, p := range a.Outputs() {
		var values []value
		for _, v := range byName[p.Name].Values {
			values = append(values, g.expected(v))
		}
		lines = append(lines, held{p.Name, p.Multiplicity, values})
	}
	return g.spell(lines)
}

// renderOutputs spells a run's output parameters as renderExpected does; the
// objects they hold live in ctx.
func renderOutputs(a *Activity, ctx *runtime.Context, outputs map[string]runtime.Value) string {
	g := newGraph(a.Model, ctx)
	var lines []held
	for _, p := range a.Outputs() {
		var values []value
		if v, ok := outputs[p.Name]; ok {
			values = g.runtime(v)
		}
		lines = append(lines, held{p.Name, p.Multiplicity, values})
	}
	return g.spell(lines)
}

// value is one value the outputs reach on either side: a primitive's canonical
// spelling, or an object of the graph.
type value struct {
	text   string
	entity *entity
}

// entity is one object the outputs reach, its features in the class's attribute
// order; class is nil when the model declares no class of its type.
type entity struct {
	typeName string
	class    *Class
	features []held
	filled   bool
	arrival  int
	number   int
}

// held is a feature or parameter and the values it holds.
type held struct {
	name   string
	m      Multiplicity
	values []value
}

// graph collects the objects either side's outputs reach, keyed by that side's
// own identity, so that neither side's object ids show in the spelling.
type graph struct {
	model   *Model
	ctx     *runtime.Context
	objects map[string]*entity
	order   []*entity
}

func newGraph(model *Model, ctx *runtime.Context) *graph {
	return &graph{model: model, ctx: ctx, objects: map[string]*entity{}}
}

// at is the object of a side's identity key, added at its first mention.
func (g *graph) at(key, typeName string, c *Class) (*entity, bool) {
	if o := g.objects[key]; o != nil {
		return o, false
	}
	o := &entity{typeName: typeName, class: c, arrival: len(g.order)}
	g.objects[key] = o
	g.order = append(g.order, o)
	return o, true
}

// expected converts a recorded value.
func (g *graph) expected(v ExpectedValue) value {
	switch v.Kind {
	case "Integer", "Boolean", "String":
		var raw any
		if err := json.Unmarshal(v.Value, &raw); err == nil {
			switch raw := raw.(type) {
			case float64:
				return value{text: strconv.FormatInt(int64(raw), 10)}
			case bool:
				return value{text: strconv.FormatBool(raw)}
			case string:
				return value{text: strconv.Quote(raw)}
			}
		}
	case "Real":
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return value{text: renderReal(f)}
			}
		}
	case "Reference":
		if v.Referent != nil {
			return g.expected(*v.Referent)
		}
	case "Object":
		return value{entity: g.expectedObject(v)}
	}
	return value{text: v.Kind + string(v.Value)}
}

// expectedObject converts a recorded object by the class its type names; the
// record repeats the features at every mention, so the first untruncated one fills them.
func (g *graph) expectedObject(v ExpectedValue) *entity {
	var c *Class
	if len(v.Types) == 1 {
		c = g.model.ClassOf(TypeRef{Name: v.Types[0]})
	}
	o, _ := g.at(v.ID, strings.Join(v.Types, "&"), c)
	if o.filled || v.Truncated || o.class == nil {
		return o
	}
	o.filled = true
	recorded := map[string][]ExpectedValue{}
	for _, f := range v.Features {
		recorded[f.Feature] = f.Values
	}
	for _, attr := range sortedAttributes(o.class) {
		var values []value
		for _, rv := range recorded[attr.Name] {
			values = append(values, g.expected(rv))
		}
		o.features = append(o.features, held{attr.Name, attr.Multiplicity, values})
	}
	return o
}

// runtime converts a run's value, a sequence as its elements.
func (g *graph) runtime(v runtime.Value) []value {
	if seq := v.Sequence(); seq != nil {
		var out []value
		for _, e := range seq.Elements() {
			out = append(out, g.runtime(e)...)
		}
		return out
	}
	switch v.Kind {
	case runtime.ValNull:
		return nil
	case runtime.ValString:
		return []value{{text: strconv.Quote(v.Str())}}
	case runtime.ValConst:
		switch v.Const.Kind {
		case semantics.ValInt:
			return []value{{text: strconv.FormatInt(v.Const.Int, 10)}}
		case semantics.ValBool:
			return []value{{text: strconv.FormatBool(v.Const.Bool)}}
		case semantics.ValReal:
			return []value{{text: renderReal(v.Const.Real)}}
		}
	case runtime.ValInstance:
		// A valueless feature of a value type is materialized as an object that
		// is no value, which every surface of the runtime reads as unset.
		if g.ctx.HoldsNoValue(v) {
			return nil
		}
		return []value{{entity: g.runtimeObject(v.Instance)}}
	}
	return []value{{text: runtime.FormatValue(v)}}
}

// runtimeObject converts a run's object by the class its definition translates.
func (g *graph) runtimeObject(id int64) *entity {
	key := "#" + strconv.FormatInt(id, 10)
	inst, ok := g.ctx.Instance(id)
	if !ok || inst.Type == nil {
		o, _ := g.at(key, "<unknown object>", nil)
		return o
	}
	o, fresh := g.at(key, inst.Type.Name, g.model.ClassOf(TypeRef{Name: inst.Type.Name}))
	if !fresh || o.class == nil {
		return o
	}
	o.filled = true
	for _, attr := range sortedAttributes(o.class) {
		fv, err := inst.GetFeatureValue(g.ctx, attr.Name)
		if err != nil {
			o.features = append(o.features, held{attr.Name, attr.Multiplicity, []value{{text: "<error: " + err.Error() + ">"}}})
			continue
		}
		var values []value
		switch {
		case !fv.Feature.Scalar():
			if fv.Values.Kind != runtime.ValInvalid {
				values = g.runtime(fv.Values)
			}
		case fv.Materialized && fv.Value.Kind != runtime.ValInvalid:
			values = g.runtime(fv.Value)
		}
		o.features = append(o.features, held{attr.Name, attr.Multiplicity, values})
	}
	return o
}

// spell numbers the objects canonically and spells the lines, an object as
// `Type#n{feature = values; …}` at its first mention and `#n` after.
func (g *graph) spell(lines []held) string {
	g.number(lines)
	s := &speller{shown: map[*entity]bool{}}
	var out []string
	for _, l := range lines {
		out = append(out, s.line(l))
	}
	return strings.Join(out, "\n")
}

// number aliases the objects 1..n by type, features and holders, refined until the
// classes settle; objects the refinement cannot tell apart keep first-mention order.
func (g *graph) number(lines []held) {
	class := map[*entity]string{}
	for _, o := range g.order {
		class[o] = o.typeName
	}
	distinct := 0
	for range g.order {
		next := g.refine(lines, class)
		n := len(ranks(next))
		if n <= distinct {
			break
		}
		distinct, class = n, next
	}
	order := append([]*entity(nil), g.order...)
	sort.SliceStable(order, func(i, j int) bool {
		if class[order[i]] != class[order[j]] {
			return class[order[i]] < class[order[j]]
		}
		return order[i].arrival < order[j].arrival
	})
	for i, o := range order {
		o.number = i + 1
	}
}

// refine is one round: an object's new class is its type, the classes its features
// hold and its holders' classes and features, compressed to a rank to stay bounded.
func (g *graph) refine(lines []held, class map[*entity]string) map[*entity]string {
	holders := map[*entity][]string{}
	hold := func(holder string, fs []held) {
		for _, f := range fs {
			for i, v := range f.values {
				if v.entity == nil {
					continue
				}
				at := ""
				if !unordered(f.m) {
					at = "@" + strconv.Itoa(i)
				}
				holders[v.entity] = append(holders[v.entity], holder+"."+f.name+at)
			}
		}
	}
	hold("", lines)
	for _, o := range g.order {
		hold(class[o], o.features)
	}
	next := map[*entity]string{}
	for _, o := range g.order {
		var fs []string
		for _, f := range o.features {
			var vs []string
			for _, v := range f.values {
				if v.entity != nil {
					vs = append(vs, "#"+class[v.entity])
				} else {
					vs = append(vs, v.text)
				}
			}
			if unordered(f.m) {
				sort.Strings(vs)
			}
			fs = append(fs, f.name+"="+strings.Join(vs, ","))
		}
		in := holders[o]
		sort.Strings(in)
		next[o] = o.typeName + "{" + strings.Join(fs, ";") + "}<" + strings.Join(in, ",") + ">"
	}
	rank := ranks(next)
	for o, c := range next {
		next[o] = strconv.Itoa(rank[c])
	}
	return next
}

// ranks numbers the distinct classes in sorted order.
func ranks(class map[*entity]string) map[string]int {
	var names []string
	seen := map[string]bool{}
	for _, c := range class {
		if !seen[c] {
			seen[c] = true
			names = append(names, c)
		}
	}
	sort.Strings(names)
	rank := map[string]int{}
	for i, n := range names {
		rank[n] = i
	}
	return rank
}

// speller writes the numbered graph out, each object in full once.
type speller struct {
	shown map[*entity]bool
}

// line spells one feature's values: absent as `-`, an unordered feature's in key order.
func (s *speller) line(h held) string {
	if len(h.values) == 0 {
		return h.name + " = -"
	}
	values := append([]value(nil), h.values...)
	if unordered(h.m) {
		sort.SliceStable(values, func(i, j int) bool { return values[i].key() < values[j].key() })
	}
	var parts []string
	for _, v := range values {
		parts = append(parts, s.value(v))
	}
	return h.name + " = " + strings.Join(parts, ", ")
}

// key orders an unordered feature's values: a primitive by spelling, an object by alias.
func (v value) key() string {
	if v.entity != nil {
		return "#" + strconv.Itoa(v.entity.number)
	}
	return v.text
}

func (s *speller) value(v value) string {
	o := v.entity
	if o == nil {
		return v.text
	}
	alias := "#" + strconv.Itoa(o.number)
	if s.shown[o] {
		return alias
	}
	s.shown[o] = true
	if o.class == nil {
		return o.typeName + alias + "{?}"
	}
	var lines []string
	for _, f := range o.features {
		lines = append(lines, s.line(f))
	}
	return o.typeName + alias + "{" + strings.Join(lines, "; ") + "}"
}

// unordered reports whether a feature's values form a multiset rather than a list.
func unordered(m Multiplicity) bool {
	return !m.Ordered && m.Upper != 1
}

// sortedAttributes is the class's own and inherited attributes in name order.
func sortedAttributes(c *Class) []*Property {
	attrs := append([]*Property(nil), c.AllAttributes()...)
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name < attrs[j].Name })
	return attrs
}

// renderReal spells a real so that the implementation's and the runtime's agree
// when they are the same number.
func renderReal(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Sprint(f)
	}
	return strconv.FormatFloat(f, 'g', 15, 64)
}
