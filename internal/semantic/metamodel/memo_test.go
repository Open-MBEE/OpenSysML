package metamodel

import (
	"reflect"
	"testing"
	"unsafe"
)

// A memo entry holds a value in fewer words than the value itself, so every field
// of a value has to come back out of it: one set alone, each in turn, and all at once.
func TestAMemoEntryServesTheValueItWasGiven(t *testing.T) {
	samples := map[string]Value{"zero": {}, "null": nullValue()}
	full := Value{}
	fields := reflect.TypeOf(Value{})
	for i := range fields.NumField() {
		value := Value{}
		for _, target := range []reflect.Value{reflect.ValueOf(&value).Elem().Field(i), reflect.ValueOf(&full).Elem().Field(i)} {
			setTestField(t, fields.Field(i).Name, target)
		}
		samples[fields.Field(i).Name] = value
	}
	samples["every field"] = full
	evaluator := &Evaluator{}
	for name, want := range samples {
		key := Element{Aspect: name}.Key()
		stored, handled, ok := evaluator.memoize(key, memoValueList, name, want, true, false)
		if !reflect.DeepEqual(stored, want) || !handled || ok {
			t.Errorf("%s: memoize returned %#v, %t, %t", name, stored, handled, ok)
		}
		got, handled, ok, known := evaluator.memoized(key, memoValueList, name)
		if !known || !reflect.DeepEqual(got, want) || !handled || ok {
			t.Errorf("%s: memoized = %#v, %t, %t, %t; want %#v", name, got, handled, ok, known, want)
		}
		if _, _, _, known := evaluator.memoized(key, memoName, name); known {
			t.Errorf("%s: a memo of one derivation answers for another", name)
		}
	}
}

// The first result stored for a derivation is the one every later caller is served.
func TestTheFirstMemoizedResultStands(t *testing.T) {
	evaluator := &Evaluator{}
	key := Element{Aspect: "owner"}.Key()
	first := Value{Kind: StringValue, String: "first", Success: true}
	evaluator.memoize(key, memoName, "name", first, true, true)
	got, _, _ := evaluator.memoize(key, memoName, "name", Value{Kind: StringValue, String: "second", Success: true}, true, true)
	if !reflect.DeepEqual(got, first) {
		t.Fatalf("a second memoize served %#v; want the stored %#v", got, first)
	}
}

// An entry is kept for every derivation of every element, so it must stay well
// under the value it stands for.
func TestAMemoEntryIsSmallerThanAValue(t *testing.T) {
	if entry, value := unsafe.Sizeof(memoEntry{}), unsafe.Sizeof(Value{}); entry*3 > value {
		t.Fatalf("a memo entry is %d bytes beside a value of %d", entry, value)
	}
}

// A property no rule serves is answered from the rule table alone: asking for it
// on every element of a model must not leave an entry per element behind.
func TestAPropertyWithoutARuleLeavesNothingPerElement(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "no-rule.sysml", "package P {}")
	values, reasons, memos := len(evaluator.cache), len(evaluator.reasons), len(evaluator.memos)
	for _, aspect := range []string{"a", "b", "c"} {
		element := Element{Aspect: aspect}
		if _, ok := evaluator.Property(element, "CalculationDefinition", "calculation"); ok {
			t.Fatal("a property without a rule succeeded")
		}
		want := Reason{Category: ReasonNoRule, Detail: "CalculationDefinition::calculation"}
		if got, ok := evaluator.Explain(element, "CalculationDefinition", "calculation"); !ok || got != want {
			t.Fatalf("Explain = %#v, %t; want %#v", got, ok, want)
		}
	}
	if len(evaluator.cache) != values || len(evaluator.reasons) != reasons || len(evaluator.memos) != memos {
		t.Fatalf("the evaluator kept %d values, %d reasons and %d memos for a property without a rule",
			len(evaluator.cache)-values, len(evaluator.reasons)-reasons, len(evaluator.memos)-memos)
	}
}

func setTestField(t *testing.T, name string, field reflect.Value) {
	t.Helper()
	switch field.Interface().(type) {
	case ValueKind:
		field.Set(reflect.ValueOf(StringValue))
	case Element:
		field.Set(reflect.ValueOf(Element{Aspect: "element"}))
	case Membership:
		field.Set(reflect.ValueOf(Membership{Aspect: "membership"}))
	case string:
		field.SetString("text")
	case bool:
		field.SetBool(true)
	case int64:
		field.SetInt(-3)
	case float64:
		field.SetFloat(2.5)
	case []Value:
		field.Set(reflect.ValueOf([]Value{{Kind: BooleanValue, Boolean: true, Success: true}}))
	default:
		t.Fatalf("Value.%s has a type this test does not know how to set", name)
	}
}
