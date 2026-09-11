package symbols

import (
	"slices"
	"sync/atomic"
)

type indexGeneration struct {
	value atomic.Uint64
}

func (g *indexGeneration) bump() {
	g.value.Add(1)
}

func (g *indexGeneration) get() uint64 {
	return g.value.Load()
}

// layer is one of the index's tables, which may read through to the same table
// of a frozen index below it. A write lands in own, so the layer below is shared
// unmodified between every index built over it, and a key the index above
// deletes is remembered in dead rather than removed from it.
//
// A plain index has no layer below and writes straight into own, so it costs one
// map lookup more than the bare map it replaces and nothing else.
type layer[K comparable, V any] struct {
	base map[K]V    // the frozen index's table, never written
	own  map[K]V    // this index's own entries
	dead map[K]bool // base keys this index hides
	gen  *indexGeneration
}

// newLayer returns an empty table with nothing below it.
func newLayer[K comparable, V any](gen *indexGeneration) *layer[K, V] {
	return &layer[K, V]{own: make(map[K]V), gen: gen}
}

// overLayer returns an empty table reading through to below's entries.
func overLayer[K comparable, V any](below *layer[K, V], gen *indexGeneration) *layer[K, V] {
	return &layer[K, V]{base: below.own, own: make(map[K]V), gen: gen}
}

// get returns the entry under k, preferring this layer's own over the one below
// and reporting a key this layer deleted as absent.
func (l *layer[K, V]) get(k K) (V, bool) {
	if v, ok := l.own[k]; ok {
		return v, true
	}
	if l.dead[k] {
		var zero V
		return zero, false
	}
	v, ok := l.base[k]
	return v, ok
}

// at returns the entry under k, or the zero value when there is none.
func (l *layer[K, V]) at(k K) V {
	v, _ := l.get(k)
	return v
}

// below returns the entry the layer below holds under k, for a caller that has
// already found no own entry there.
func (l *layer[K, V]) below(k K) (V, bool) {
	if l.dead[k] {
		var zero V
		return zero, false
	}
	v, ok := l.base[k]
	return v, ok
}

// set records v under k in this layer, which also un-deletes the key.
func (l *layer[K, V]) set(k K, v V) {
	l.gen.bump()
	l.own[k] = v
	delete(l.dead, k)
}

// del forgets k, hiding an entry the layer below holds under it.
func (l *layer[K, V]) del(k K) {
	l.gen.bump()
	delete(l.own, k)
	if _, ok := l.base[k]; ok {
		if l.dead == nil {
			l.dead = make(map[K]bool)
		}
		l.dead[k] = true
	}
}

// owns reports whether the entry under k belongs to this layer, so a caller
// about to write into a map or slice value knows whether it is shared.
func (l *layer[K, V]) owns(k K) bool {
	_, ok := l.own[k]
	return ok
}

// keys returns a snapshot of every key the table holds, so a caller may write to
// the table while iterating it.
func (l *layer[K, V]) keys() []K {
	out := make([]K, 0, len(l.own)+len(l.base))
	for k := range l.own {
		out = append(out, k)
	}
	for k := range l.base {
		if _, shadowed := l.own[k]; !shadowed && !l.dead[k] {
			out = append(out, k)
		}
	}
	return out
}

// clear forgets every entry, hiding the ones the layer below holds.
func (l *layer[K, V]) clear() {
	l.gen.bump()
	l.own = make(map[K]V)
	for k := range l.base {
		if l.dead == nil {
			l.dead = make(map[K]bool)
		}
		l.dead[k] = true
	}
}

// writableMap returns the map under k that this layer may write to, copying a
// map the layer below owns first: one index's document must not appear in
// another's through a table they share.
func writableMap[K comparable, NK comparable, NV any](l *layer[K, map[NK]NV], k K) map[NK]NV {
	if m, owned := l.own[k]; owned {
		l.gen.bump()
		return m
	}
	shared, _ := l.below(k)
	out := make(map[NK]NV, len(shared)+1)
	for nk, nv := range shared {
		out[nk] = nv
	}
	l.set(k, out)
	return out
}

// writableSlice returns the slice under k that this layer may append to, copying
// one the layer below owns — appending to it could write into the shared array.
func writableSlice[K comparable, S ~[]E, E any](l *layer[K, S], k K) S {
	if s, owned := l.own[k]; owned {
		l.gen.bump()
		return s
	}
	shared, _ := l.below(k)
	out := make(S, len(shared), len(shared)+1)
	copy(out, shared)
	l.set(k, out)
	return out
}

// appendSlice appends e to the slice under k, copying a slice the layer below
// owns first (see writableSlice).
func appendSlice[K comparable, S ~[]E, E any](l *layer[K, S], k K, e E) {
	if s, owned := l.own[k]; owned {
		l.gen.bump()
		l.own[k] = append(s, e)
		return
	}
	shared, _ := l.below(k)
	out := make(S, len(shared), len(shared)+1)
	copy(out, shared)
	l.set(k, append(out, e))
}

// insertSorted adds s to the sorted, duplicate-free slice under k, copying a
// slice the layer below owns first (see writableSlice).
func insertSorted(l *layer[string, []string], k, s string) {
	if owned, ok := l.own[k]; ok {
		i, found := slices.BinarySearch(owned, s)
		if !found {
			l.gen.bump()
			l.own[k] = slices.Insert(owned, i, s)
		}
		return
	}
	have, _ := l.below(k)
	i, found := slices.BinarySearch(have, s)
	if found {
		return
	}
	out := make([]string, len(have)+1)
	copy(out, have[:i])
	out[i] = s
	copy(out[i+1:], have[i:])
	l.set(k, out)
}

// removeSorted drops s from the sorted slice under k and returns what remains.
func removeSorted(l *layer[string, []string], k, s string) []string {
	have, _ := l.get(k)
	i, found := slices.BinarySearch(have, s)
	if !found {
		return have
	}
	out := writableSlice(l, k)
	out = slices.Delete(out, i, i+1)
	l.set(k, out)
	return out
}
