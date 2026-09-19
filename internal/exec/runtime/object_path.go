package runtime

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A materialization path names an object independently of its run's numbering: a
// root by type and creation rank, `Plant::tank#1`; a held object by its holder's
// path and the holding feature, indexed in a collection, `Plant::plant#1.tanks[0]`.

// objectPath is the object's materialization path; "<unknown object>" for an
// identity the context does not hold.
func (ctx *Context) objectPath(id int64) string {
	inst, ok := ctx.Instance(id)
	if !ok {
		return "<unknown object>"
	}
	owner, feature := inst.Owner()
	if owner == nil {
		return ctx.rankedRootPath(inst)
	}
	path := ctx.objectPath(owner.ID) + "." + feature
	if fv := owner.FeatureValues[feature]; fv != nil && !fv.Feature.Scalar() {
		if i := ctx.memberIndex(fv.Values, id); i >= 0 {
			path += "[" + strconv.Itoa(i) + "]"
		}
	}
	return path
}

// rankedRootPath names an object no other holds by its type and its rank, from 1,
// among the roots of that type, in the order the run made them.
func (ctx *Context) rankedRootPath(inst *Instance) string {
	roots := func(other *Instance) bool {
		owner, _ := other.Owner()
		return owner == nil && other.Type == inst.Type
	}
	return fmt.Sprintf("%s#%d", ctx.typeName(inst), ctx.creationRank(inst.ID, roots)+1)
}

// memberIndex is the object's place among the objects a collection holds: its
// position in a sequence, its creation rank among a set's members; -1 if the
// collection does not hold it.
func (ctx *Context) memberIndex(collection Value, id int64) int {
	holds := func(v Value) bool { return v.Kind == ValInstance && v.Instance == id }
	if collection.Kind != ValSet {
		return slices.IndexFunc(elementsOf(collection), holds)
	}
	members := setMembers(collection)
	if !members[id] {
		return -1
	}
	return ctx.creationRank(id, func(other *Instance) bool { return members[other.ID] })
}

// creationRank is the object's rank, from 0, among the objects satisfying among,
// in the order the run made them; -1 if the run made no such object.
func (ctx *Context) creationRank(id int64, among func(*Instance) bool) int {
	rank := 0
	for _, created := range ctx.created {
		other, ok := ctx.Instance(created)
		if !ok || !among(other) {
			continue
		}
		if created == id {
			return rank
		}
		rank++
	}
	return -1
}

// rankedAmong is the object of rank n, from 0, among those satisfying among, in
// the order the run made them; nil if the run made fewer.
func (ctx *Context) rankedAmong(n int, among func(*Instance) bool) *Instance {
	for _, created := range ctx.created {
		inst, ok := ctx.Instance(created)
		if !ok || !among(inst) {
			continue
		}
		if n == 0 {
			return inst
		}
		n--
	}
	return nil
}

func (ctx *Context) typeName(inst *Instance) string {
	if inst.Type == nil {
		return "object"
	}
	return ctx.symbolName(inst.Type)
}

func (ctx *Context) symbolName(sym *symbols.Symbol) string {
	if fqn := ctx.fqnOf(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}

// objectAt is the object at a materialization path, as objectPath spells one; the
// error says which part of the path this run made no object for.
func (ctx *Context) objectAt(path string) (*Instance, error) {
	root, rest, _ := strings.Cut(path, ".")
	typeName, digits, ranked := strings.Cut(root, "#")
	rank, err := strconv.Atoi(digits)
	if !ranked || err != nil || rank < 1 {
		return nil, fmt.Errorf("%q names no root: a root is spelled <type>#<rank>", root)
	}
	inst := ctx.rankedAmong(rank-1, func(other *Instance) bool {
		owner, _ := other.Owner()
		return owner == nil && ctx.typeName(other) == typeName
	})
	if inst == nil {
		return nil, fmt.Errorf("the run made no %s", root)
	}
	for at := root; rest != ""; {
		var segment string
		segment, rest, _ = strings.Cut(rest, ".")
		at += "." + segment
		if inst, err = ctx.heldAt(inst, segment); err != nil {
			return nil, fmt.Errorf("the run made no %s: %w", at, err)
		}
	}
	return inst, nil
}

// heldAt is the object the holder's feature holds, as a path segment names it:
// the feature, indexed as `feature[i]` within a collection.
func (ctx *Context) heldAt(holder *Instance, segment string) (*Instance, error) {
	feature, index := segment, -1
	if name, after, indexed := strings.Cut(segment, "["); indexed {
		digits, closed := strings.CutSuffix(after, "]")
		i, err := strconv.Atoi(digits)
		if !closed || err != nil || i < 0 {
			return nil, fmt.Errorf("%q indexes no collection: an index is spelled [<n>]", segment)
		}
		feature, index = name, i
	}
	fv := holder.FeatureValues[feature]
	if fv == nil {
		return nil, fmt.Errorf("%s has no feature %s", ctx.typeName(holder), feature)
	}
	if fv.Feature.Scalar() {
		if index >= 0 {
			return nil, fmt.Errorf("%s holds one value, not a collection", feature)
		}
		return ctx.heldObject(fv.Value)
	}
	if index < 0 {
		return nil, fmt.Errorf("%s holds a collection; index it", feature)
	}
	if fv.Values.Kind != ValSet {
		elements := elementsOf(fv.Values)
		if index >= len(elements) {
			return nil, fmt.Errorf("%s holds %d values", feature, len(elements))
		}
		return ctx.heldObject(elements[index])
	}
	members := setMembers(fv.Values)
	inst := ctx.rankedAmong(index, func(other *Instance) bool { return members[other.ID] })
	if inst == nil {
		return nil, fmt.Errorf("%s holds %d objects", feature, len(members))
	}
	return inst, nil
}

// heldObject is the object a value is, or an error for a value that is none.
func (ctx *Context) heldObject(v Value) (*Instance, error) {
	if v.Kind != ValInstance {
		return nil, fmt.Errorf("holds %s, not an object", FormatTraceValue(v))
	}
	inst, ok := ctx.Instance(v.Instance)
	if !ok {
		return nil, fmt.Errorf("holds object #%d, which the run no longer has", v.Instance)
	}
	return inst, nil
}

// setMembers are the identities of the objects a set holds.
func setMembers(set Value) map[int64]bool {
	members := make(map[int64]bool)
	for _, element := range elementsOf(set) {
		if element.Kind == ValInstance {
			members[element.Instance] = true
		}
	}
	return members
}
