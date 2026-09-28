package passes

import (
	"fmt"
	"strings"
	"testing"
)

// partUsageFindings returns "line: message" for each
// part-usage-part-definition diagnostic src draws, in order.
func partUsageFindings(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, d := range only(w8cLibraryDiagnostics(t, "<t>.sysml", src), "part-usage-part-definition") {
		out = append(out, fmt.Sprintf("%d: %s", strings.Count(src[:d.Span.Offset], "\n")+1, d.Message))
	}
	return out
}

func TestPartUsagePartDefinitionRejectsNonPartTypes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "issue: part typed by an item definition",
			src: `package P {
	item def Start;
	part def L {
		part bread : Start;
	}
}`,
			want: []string{"4: " + msgPartUsagePartDefinition},
		},
		{
			name: "subsetting an item-typed part",
			src: `item def I;
part def D {
	part a : I;
	part b :> a;
}`,
			want: []string{"3: " + msgPartUsagePartDefinition, "4: " + msgPartUsagePartDefinition},
		},
		{
			name: "subsetting an item usage typed by an item def",
			src: `item def I;
item i : I;
part p :> i;`,
			want: []string{"3: " + msgPartUsagePartDefinition},
		},
		{
			name: "redefinition inheriting only an item def",
			src: `item def I;
part def A {
	part x : I;
}
part def B :> A {
	part :>> x;
}`,
			want: []string{"3: " + msgPartUsagePartDefinition, "6: " + msgPartUsagePartDefinition},
		},
		{
			name: "variant part typed by an item def",
			src: `item def I;
part def PD;
part p : PD {
	variation part vp {
		variant part v : I;
	}
}`,
			want: []string{"5: " + msgPartUsagePartDefinition},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := partUsageFindings(t, tc.src)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("part-usage-part-definition = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPartUsagePartDefinitionAcceptsPartTyping(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{name: "untyped part", src: `part def PD;
part p;`},
		{name: "typed by a part def", src: `part def PD;
part p : PD;`},
		{name: "typed by a part def and an item def", src: `item def I;
part def PD;
part p : I, PD;`},
		{name: "inheriting a part def through subsetting", src: `part def PD;
part q : PD;
part r :> q;
item def I;
part s : I :> q;`},
		{name: "redefinition inheriting a part def", src: `part def PD;
part def A {
	part x : PD;
}
part def B :> A {
	part :>> x;
}`},
		{name: "untyped part subsetting an untyped part", src: `part a;
part b :> a;`},
		{name: "unresolved type", src: `part p : Nope;`},
		{name: "item usage typed by an item def", src: `item def I;
item i : I;`},
		{name: "typed by a connection def", src: `connection def CD;
part c : CD;`},
		{name: "typed by an interface def", src: `interface def ID;
part i : ID;`},
		{name: "typed by a view def", src: `view def VD;
part v : VD;`},
		{name: "inheriting a connection def through subsetting", src: `connection def CD;
part c : CD;
part d :> c;`},
		{name: "bare variant reference of a variation action", src: `action def AD;
action a {
	variation action va : AD {
		variant v1;
	}
}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := partUsageFindings(t, tc.src); len(got) != 0 {
				t.Errorf("part-usage-part-definition = %v, want none", got)
			}
		})
	}
}
