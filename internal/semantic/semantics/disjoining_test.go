package semantics

import "testing"

// A union's instances are exactly those of its unioning types, so a union
// conforms to every type all of them conform to (KerML 1.1 §7.3.2.3).
func TestConformsThroughUnion(t *testing.T) {
	for _, name := range []string{"a.kerml", "a.sysml"} {
		m, root := buildModelNamed(t, name, `
			classifier Wheel;
			classifier MyWheel1 specializes Wheel;
			classifier MyWheel2 specializes Wheel;
			classifier MyWheel unions MyWheel1, MyWheel2;
			classifier Sprocket;
			classifier Mixed unions MyWheel1, Sprocket;
			classifier Loop1 unions MyWheel1, Loop2;
			classifier Loop2 unions Loop1, Sprocket;`)
		wheel := nested(t, root, "Wheel")
		if !m.Conforms(nested(t, root, "MyWheel"), wheel) {
			t.Errorf("%s: MyWheel (unions two Wheels) does not conform to Wheel", name)
		}
		if m.Conforms(nested(t, root, "Mixed"), wheel) {
			t.Errorf("%s: Mixed (unions a Wheel and a Sprocket) conforms to Wheel", name)
		}
		if m.Conforms(nested(t, root, "Loop1"), wheel) {
			t.Errorf("%s: Loop1 (a unioning cycle reaching a Sprocket) conforms to Wheel", name)
		}
	}
}

// Disjoint is symmetric, descends to subtypes and through unions (a unioning cycle
// establishes nothing), and never follows from mere unrelatedness.
func TestDisjoint(t *testing.T) {
	m, root := buildModelNamed(t, "a.kerml", `
		class A; class B disjoint from A; class C;
		class A1 :> A; class B1 :> B;
		class U unions A1, C;
		class D disjoint from U;
		class AB unions A1, B1;
		class ABSub :> AB;
		class Loop1 unions Loop2, A1; class Loop2 unions Loop1;`)
	cases := []struct {
		a, b string
		want bool
	}{
		{"A", "B", true}, {"B", "A", true},
		{"A1", "B1", true}, {"B1", "A1", true},
		{"A", "C", false}, {"A1", "C", false}, {"A", "A1", false}, {"A", "A", false},
		{"A1", "D", true}, {"C", "D", true}, {"D", "A1", true},
		{"AB", "C", false}, {"ABSub", "C", false},
		{"Loop1", "B", false}, {"Loop2", "B", false},
	}
	for _, c := range cases {
		if got := m.Disjoint(nested(t, root, c.a), nested(t, root, c.b)); got != c.want {
			t.Errorf("Disjoint(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
