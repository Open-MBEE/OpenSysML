// Package graphcmp compares two pointer graphs structurally, for tests: same
// values, same shape, same sharing, and nil kept apart from empty.
package graphcmp

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Option adjusts a comparison.
type Option func(*comparer)

// SkipFields excludes struct fields, named "Type.field", from the comparison:
// caches and other per-process state the graphs need not agree on.
func SkipFields(names ...string) Option {
	return func(c *comparer) {
		for _, n := range names {
			c.skip[n] = true
		}
	}
}

// Equal reports the first difference between the graphs reachable from want
// and got, or nil when there is none.
func Equal(want, got any, opts ...Option) error {
	c := &comparer{
		skip: map[string]bool{},
		fwd:  map[ref]uintptr{},
		rev:  map[ref]uintptr{},
	}
	for _, o := range opts {
		o(c)
	}
	if err := c.walk(reflect.ValueOf(want), reflect.ValueOf(got)); err != nil {
		return err
	}
	// Maps keyed by pointers are matched once every pointer has its partner.
	for len(c.deferred) > 0 {
		d := c.deferred[0]
		c.deferred = c.deferred[1:]
		c.path = d.path
		if err := c.pointerKeyedMap(d.a, d.b); err != nil {
			return err
		}
	}
	return nil
}

// ref identifies an object by type and address, since a struct and its first
// field share an address.
type ref struct {
	t reflect.Type
	p uintptr
}

type deferredMap struct {
	a, b reflect.Value
	path []string
}

type comparer struct {
	skip     map[string]bool
	fwd, rev map[ref]uintptr // want -> got and got -> want object pairing
	deferred []deferredMap
	path     []string
}

func (c *comparer) fail(format string, args ...any) error {
	return fmt.Errorf("%s: %s", strings.Join(c.path, ""), fmt.Sprintf(format, args...))
}

func (c *comparer) failValues(a, b any) error { return c.fail("%v vs %v", a, b) }

func (c *comparer) failNil(a, b reflect.Value) error {
	return c.fail("nil %v vs %v", a.IsNil(), b.IsNil())
}

func (c *comparer) push(p string) { c.path = append(c.path, p) }
func (c *comparer) pop()          { c.path = c.path[:len(c.path)-1] }

func (c *comparer) walk(a, b reflect.Value) error {
	if a.Kind() != b.Kind() {
		return c.fail("kind %v vs %v", a.Kind(), b.Kind())
	}
	switch a.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Pointer:
		return c.pointer(a, b)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				return c.failNil(a, b)
			}
			return nil
		}
		if a.Elem().Type() != b.Elem().Type() {
			return c.fail("dynamic type %v vs %v", a.Elem().Type(), b.Elem().Type())
		}
		return c.walk(a.Elem(), b.Elem())
	case reflect.Struct:
		return c.structs(a, b)
	case reflect.Slice, reflect.Array:
		if a.Kind() == reflect.Slice && a.IsNil() != b.IsNil() {
			return c.fail("nil slice %v vs %v", a.IsNil(), b.IsNil())
		}
		if a.Len() != b.Len() {
			return c.fail("len %d vs %d", a.Len(), b.Len())
		}
		for i := 0; i < a.Len(); i++ {
			c.push("[" + strconv.Itoa(i) + "]")
			if err := c.walk(a.Index(i), b.Index(i)); err != nil {
				return err
			}
			c.pop()
		}
		return nil
	case reflect.Map:
		if a.IsNil() != b.IsNil() {
			return c.fail("nil map %v vs %v", a.IsNil(), b.IsNil())
		}
		if a.Len() != b.Len() {
			return c.fail("len %d vs %d", a.Len(), b.Len())
		}
		if hasPointers(a.Type().Key()) {
			c.deferred = append(c.deferred, deferredMap{a, b, append([]string(nil), c.path...)})
			return nil
		}
		it := a.MapRange()
		for it.Next() {
			bv := b.MapIndex(it.Key())
			c.push("[" + fmt.Sprint(it.Key()) + "]")
			if !bv.IsValid() {
				return c.fail("key missing")
			}
			if err := c.walk(it.Value(), bv); err != nil {
				return err
			}
			c.pop()
		}
		return nil
	case reflect.String:
		if a.String() != b.String() {
			return c.fail("%q vs %q", a.String(), b.String())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			return c.failValues(a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			return c.fail("%d vs %d", a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if a.Uint() != b.Uint() {
			return c.fail("%d vs %d", a.Uint(), b.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if a.Float() != b.Float() {
			return c.failValues(a.Float(), b.Float())
		}
	case reflect.Complex64, reflect.Complex128:
		if a.Complex() != b.Complex() {
			return c.failValues(a.Complex(), b.Complex())
		}
	default: // Chan, Func, UnsafePointer
		if a.IsNil() != b.IsNil() {
			return c.failNil(a, b)
		}
		if !a.IsNil() {
			return c.fail("%v values cannot be compared", a.Kind())
		}
	}
	return nil
}

func (c *comparer) pointer(a, b reflect.Value) error {
	if a.IsNil() || b.IsNil() {
		if a.IsNil() != b.IsNil() {
			return c.failNil(a, b)
		}
		return nil
	}
	ka, kb := ref{a.Type(), a.Pointer()}, ref{b.Type(), b.Pointer()}
	if p, ok := c.fwd[ka]; ok {
		if p != kb.p {
			return c.fail("one %v object on the left is two on the right", a.Type())
		}
		return nil
	}
	if _, ok := c.rev[kb]; ok {
		return c.fail("two %v objects on the left are one on the right", a.Type())
	}
	c.fwd[ka], c.rev[kb] = kb.p, ka.p
	c.push("*")
	if err := c.walk(a.Elem(), b.Elem()); err != nil {
		return err
	}
	c.pop()
	return nil
}

func (c *comparer) structs(a, b reflect.Value) error {
	t := a.Type()
	if t.PkgPath() == "sync" || t.PkgPath() == "sync/atomic" {
		return nil
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if c.skip[t.Name()+"."+f.Name] {
			continue
		}
		c.push("." + f.Name)
		if err := c.walk(a.Field(i), b.Field(i)); err != nil {
			return err
		}
		c.pop()
	}
	return nil
}

// pointerKeyedMap matches the keys of two maps by translating the left map's
// pointers to their partners, then compares the values.
func (c *comparer) pointerKeyedMap(a, b reflect.Value) error {
	byKey := make(map[string]reflect.Value, b.Len())
	it := b.MapRange()
	for it.Next() {
		s, err := c.keyString(it.Key(), false)
		if err != nil {
			return err
		}
		byKey[s] = it.Value()
	}
	it = a.MapRange()
	for it.Next() {
		s, err := c.keyString(it.Key(), true)
		if err != nil {
			return err
		}
		bv, ok := byKey[s]
		c.push("[" + s + "]")
		if !ok {
			return c.fail("key missing")
		}
		if err := c.walk(it.Value(), bv); err != nil {
			return err
		}
		c.pop()
	}
	return nil
}

// keyString renders a map key, with its pointers translated to the right-hand
// graph when translate is set.
func (c *comparer) keyString(k reflect.Value, translate bool) (string, error) {
	switch k.Kind() {
	case reflect.Pointer:
		if k.IsNil() {
			return "nil", nil
		}
		p := k.Pointer()
		if translate {
			var ok bool
			if p, ok = c.fwd[ref{k.Type(), p}]; !ok {
				return "", c.fail("%v key never reached through the graph", k.Type())
			}
		}
		return "0x" + strconv.FormatUint(uint64(p), 16), nil
	case reflect.Interface:
		if k.IsNil() {
			return "nil", nil
		}
		s, err := c.keyString(k.Elem(), translate)
		return k.Elem().Type().String() + "(" + s + ")", err
	case reflect.Struct:
		parts := make([]string, k.NumField())
		for i := range parts {
			s, err := c.keyString(k.Field(i), translate)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	case reflect.String:
		return strconv.Quote(k.String()), nil
	default:
		return fmt.Sprint(k), nil
	}
}

func hasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface:
		return true
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if hasPointers(t.Field(i).Type) {
				return true
			}
		}
	case reflect.Array:
		return hasPointers(t.Elem())
	}
	return false
}
