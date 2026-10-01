package ast

import "reflect"

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
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if t.Field(i).IsExported() {
				inspectValue(v.Field(i), visit, seen)
			}
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
