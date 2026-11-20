package passes

import "testing"

// A header name's local namespace depends on the relationship owning it (KerML
// 8.2.3.5.2); the pinned KerML validator resolves, and fails, the same chain heads.
func TestHeaderNameLocalNamespace(t *testing.T) {
	cases := []struct {
		name, src  string
		unresolved int
	}{
		// A relationship that is no Specialization opens in the declared type's
		// own namespace, whose memberships include the inherited ones (8.2.3.5.3).
		{"disjoining inherited type", `package R { classifier Base { classifier D; } classifier C specializes Base disjoint from D; }`, 0},
		{"disjoining inherited feature", `package R { classifier Base { feature y; } classifier C { feature x : Base disjoint from y; } }`, 0},
		{"disjoining inherited chain", `package R { classifier T { feature b; } classifier Base { feature a : T; } classifier C { feature x : Base disjoint from a.b; } }`, 0},
		{"inverting inherited feature", `package R { classifier Base { feature y; } classifier C { feature x : Base inverse of y; } }`, 0},
		{"inverting inherited chain", `package R { classifier T { feature b; } classifier Base { feature a : T; } classifier C { feature x : Base inverse of a.b; } }`, 0},
		{"unioning inherited types", `package R { classifier Base { classifier D; classifier E; } classifier C specializes Base unions D, E; }`, 0},
		{"unioning inherited features", `package R { classifier Base { feature d; feature e; } classifier C { feature x : Base unions d, e; } }`, 0},
		{"intersecting inherited types", `package R { classifier Base { classifier D; classifier E; } classifier C specializes Base intersects D, E; }`, 0},
		{"differencing inherited types", `package R { classifier Base { classifier D; classifier E; } classifier C specializes Base differences D, E; }`, 0},
		{"featuring inherited feature", `package R { classifier Base { feature q; } classifier C { feature x : Base featured by q; } }`, 0},
		{"disjoining own member", `package R { classifier C disjoint from D { classifier D; } }`, 0},

		// A Specialization opens in the namespace that owns the declared type, so
		// neither the type's own members nor its inherited ones are in reach.
		{"cross subsetting own member", `package S { class X; assoc A { end feature e : X crosses q { feature q : X; } end feature f : X; } }`, 1},
		{"cross subsetting chain from own member", `package S { class X { feature p : X; }
			assoc A { end feature a : X crosses h.p { feature h : X; } end feature b : X; } }`, 1},
		{"cross subsetting chain past own namesake", `package S { class X { feature p : X; } class Y { feature r : X; }
			assoc A { end feature a : X crosses b.p { feature b : Y; } end feature b : X; } }`, 0},
		{"reference subsetting own member", `package R { classifier C { feature x references q { feature q; } } }`, 1},
		{"subsetting own member", `package R { classifier C { feature x subsets q { feature q; } } }`, 1},
		{"subsetting chain from own member", `package R { classifier T { feature b; } classifier C { feature x subsets a.b { feature a : T; } } }`, 1},
		{"subsetting chain past own namesake", `package R { classifier T { feature b; } classifier U { feature c; }
			classifier C { feature a : T; feature x subsets a.b { feature a : U; } } }`, 0},
		{"typing own member", `package R { classifier C { feature x : T { classifier T; } } }`, 1},
		{"chaining own member", `package R { classifier T { feature b; } classifier C { feature x chains a.b { feature a : T; } } }`, 1},
		{"chaining inherited member", `package R { classifier T { feature b; } classifier Base { feature a : T; } classifier C { feature x : Base chains a.b; } }`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, pd, idx := analyzeInputs(t, "header.kerml", tc.src)
			unresolved := 0
			for _, d := range Analyze("header.kerml", root, pd, idx) {
				if d.Code != "unresolved" {
					t.Errorf("unexpected diagnostic %+v", d)
					continue
				}
				unresolved++
			}
			if unresolved != tc.unresolved {
				t.Errorf("got %d unresolved references, want %d", unresolved, tc.unresolved)
			}
		})
	}
}
