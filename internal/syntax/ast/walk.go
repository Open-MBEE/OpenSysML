package ast

import (
	"reflect"
	"sync"
)

// Inspect visits n and, wherever visit answers true, every node reachable
// through n's exported fields: pointer, interface, slice and map members, and
// structs that do not themselves implement Node, so no node is missed for want
// of a field list. A node is visited at most once.
func Inspect(n Node, visit func(Node) bool) {
	inspectValue(reflect.ValueOf(n), visit, map[visitKey]bool{})
}

// visitKey identifies a visited node by its pointer, so a shared node is
// reported once and a cycle cannot loop the walk.
type visitKey struct {
	typ reflect.Type
	ptr uintptr
}

func inspectValue(v reflect.Value, visit func(Node) bool, seen map[visitKey]bool) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() || !v.CanInterface() {
			return
		}
		if node, ok := v.Interface().(Node); ok {
			if pv := reflect.ValueOf(node); pv.Kind() == reflect.Ptr {
				key := visitKey{pv.Type(), pv.Pointer()}
				if seen[key] {
					return
				}
				seen[key] = true
				if !visit(node) {
					return
				}
				inspectValue(pv.Elem(), visit, seen)
				return
			}
			if !visit(node) {
				return
			}
		}
		inspectValue(v.Elem(), visit, seen)
	case reflect.Struct:
		for _, i := range nodeFields(v.Type()) {
			inspectValue(v.Field(i), visit, seen)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			inspectValue(v.Index(i), visit, seen)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			inspectValue(v.MapIndex(k), visit, seen)
		}
	}
}

// nodeFieldPlans memoizes, per struct type, the indices of the exported fields
// a node can be reached through: reading every field's description from the
// type on each struct visited dominated a walk, and scalar fields hold no node.
var nodeFieldPlans sync.Map // reflect.Type -> []int

func nodeFields(t reflect.Type) []int {
	if plan, ok := nodeFieldPlans.Load(t); ok {
		return plan.([]int)
	}
	plan := []int{}
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() && mayHoldNode(f.Type, nil) {
			plan = append(plan, i)
		}
	}
	nodeFieldPlans.Store(t, plan)
	return plan
}

// mayHoldNode reports whether a value of type t can reach a Node: through a
// pointer, interface, slice, array or map, or a struct with such a field.
func mayHoldNode(t reflect.Type, visiting []reflect.Type) bool {
	switch t.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		return true
	case reflect.Struct:
		for _, v := range visiting {
			if v == t {
				return false
			}
		}
		visiting = append(visiting, t)
		for i := 0; i < t.NumField(); i++ {
			if f := t.Field(i); f.IsExported() && mayHoldNode(f.Type, visiting) {
				return true
			}
		}
	}
	return false
}
