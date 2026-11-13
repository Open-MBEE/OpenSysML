package metamodel

import "testing"

// identityStructure names elements by aspect up to the first '#', so two
// handles with different keys can stand for one element.
type identityStructure struct{ testStructure }

func (identityStructure) ElementIdentity(element Element) (string, bool) {
	aspect := element.Aspect
	if element.IsMembership {
		aspect = element.Membership.Aspect
	}
	for i := range aspect {
		if aspect[i] == '#' {
			return aspect[:i], true
		}
	}
	return aspect, aspect != ""
}

// A sequence keeps the first of the values repeating by key or by the structure's
// identity, in order; the copying form leaves its argument as it was given.
func TestUniqueStructureValuesKeepFirstOccurrences(t *testing.T) {
	element := func(aspect string) Value { return elementValue(Element{Aspect: aspect}) }
	membership := func(aspect string) Value {
		return Value{Kind: MembershipValue, Membership: Membership{Aspect: aspect}, Success: true}
	}
	values := []Value{
		element("a"), membership("m"), element("b#1"), element("a"), stringValue("s"),
		element("b#2"), membership("m"), membership("n#1"), stringValue("s"), membership("n#2"), element("c"),
	}
	byKey := []Value{
		element("a"), membership("m"), element("b#1"), stringValue("s"),
		element("b#2"), membership("n#1"), stringValue("s"), membership("n#2"), element("c"),
	}
	byIdentity := []Value{
		element("a"), membership("m"), element("b#1"), stringValue("s"),
		membership("n#1"), stringValue("s"), element("c"),
	}
	cases := []struct {
		name      string
		structure Structure
		want      []Value
	}{
		{"by key", testStructure{}, byKey},
		{"by identity", identityStructure{}, byIdentity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			given := slicesClone(values)
			assertSameTestValues(t, "copied", uniqueStructureValueSequence(given, tc.structure).Values, tc.want)
			assertSameTestValues(t, "argument", given, values)
			assertSameTestValues(t, "in place", uniqueOwnedStructureValues(given, tc.structure), tc.want)
		})
	}
	for _, short := range [][]Value{nil, {element("a")}} {
		assertSameTestValues(t, "short copied", uniqueStructureValueSequence(short, identityStructure{}).Values, short)
		assertSameTestValues(t, "short in place", uniqueOwnedStructureValues(short, identityStructure{}), short)
	}
}

// A memoized list is stored at its own size and never grows in place: an append
// by one reader must not reach the storage another reader holds.
func TestSharedValueListAppendsCopy(t *testing.T) {
	var shared []Value
	for size := 1; size <= 64; size++ {
		roomy := make([]Value, size, 4*size)
		for i := range roomy {
			roomy[i] = stringValue("a")
		}
		for _, given := range [][]Value{roomy, roomy[: size : size+size/4]} {
			if shared = sharedValueList(given); cap(shared) != len(shared) || len(shared) != size {
				t.Fatalf("shared list of %d values has length %d, capacity %d", size, len(shared), cap(shared))
			}
		}
	}
	first := append(shared, stringValue("first"))
	second := append(shared, stringValue("second"))
	if last := len(shared); first[last].String != "first" || second[last].String != "second" {
		t.Errorf("appends to a shared list share storage: %q, %q", first[last].String, second[last].String)
	}
	if sharedValueList(make([]Value, 0, 4)) != nil {
		t.Errorf("an empty list keeps its storage")
	}
}

func assertSameTestValues(t *testing.T, what string, got, want []Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d values, want %d", what, len(got), len(want))
	}
	for i := range want {
		same := got[i].Kind == want[i].Kind && got[i].String == want[i].String
		if same && (want[i].Kind == ElementValue || want[i].Kind == MembershipValue) {
			same = sameTestValue(got[i], want[i])
		}
		if !same {
			t.Errorf("%s: value %d differs", what, i)
		}
	}
}
