package metamodel

import "math"

// memoKind names one of the derivations the evaluator memoizes per element.
type memoKind uint8

const (
	memoName memoKind = iota
	memoQualifiedName
	memoOwnedFilter
	memoOwnedMembershipFilter
	memoOwnedMembers
	memoOwnedFeatures
	memoEffectiveFeatures
	memoNamespaceMemberships
	memoInheritedMemberships
	memoValueList
	memoSpecializedGeneralTypes
	memoFeatureDirections
)

// memoEntry is one memoized derivation of an element: a string, a list, null or nothing
// is held in place, and any other value behind a pointer, so an entry stays small.
type memoEntry struct {
	kind        memoKind
	valueKind   ValueKind
	handled, ok bool
	success     bool
	name        string
	text        string
	values      []Value
	other       *Value
}

func newMemoEntry(kind memoKind, name string, value Value, handled, ok bool) memoEntry {
	entry := memoEntry{
		kind: kind, name: name, handled: handled, ok: ok,
		valueKind: value.Kind, success: value.Success, text: value.String, values: value.Values,
	}
	if value.Element != (Element{}) || value.Membership != (Membership{}) || value.Boolean ||
		value.Integer != 0 || math.Float64bits(value.Real) != 0 || value.Enum != "" {
		entry.other = &value
	}
	return entry
}

func (m memoEntry) value() Value {
	if m.other != nil {
		return *m.other
	}
	return Value{Kind: m.valueKind, String: m.text, Values: m.values, Success: m.success}
}

// memoized returns the derivation of kind and name memoized for the element, if any.
func (e *Evaluator) memoized(key ElementKey, kind memoKind, name string) (Value, bool, bool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, entry := range e.memos[key] {
		if entry.kind == kind && entry.name == name {
			return entry.value(), entry.handled, entry.ok, true
		}
	}
	return Value{}, false, false, false
}

// memoize records a derivation of the element and returns the one on record: the
// first to be recorded stays, so every reader shares one value.
func (e *Evaluator) memoize(key ElementKey, kind memoKind, name string, value Value, handled, ok bool) (Value, bool, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	entries := e.memos[key]
	for _, entry := range entries {
		if entry.kind == kind && entry.name == name {
			return entry.value(), entry.handled, entry.ok
		}
	}
	if e.memos == nil {
		e.memos = make(map[ElementKey][]memoEntry)
	}
	e.memos[key] = append(entries, newMemoEntry(kind, name, value, handled, ok))
	return value, handled, ok
}
