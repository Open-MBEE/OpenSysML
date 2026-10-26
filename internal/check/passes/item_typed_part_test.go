package passes

import "testing"

// A part typed only by an item definition is valid: the implied Parts::parts subsetting puts
// Parts::Part in its derived partDefinition (SysML v2 §8.3.11.3, KerML §8.3.3.3.4, §7.10).
func TestPartTypedOnlyByAnItemDefinitionIsClean(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{name: "package-level part typed by an item def", src: `package P {
	item def I;
	part p : I;
}`},
		{name: "part owned by a part def, typed by an item def", src: `package P {
	item def Start;
	part def L {
		part bread : Start;
	}
}`},
		{name: "part owned by an item def, typed by its owner", src: `item def I {
	part i : I;
}`},
		{name: "abstract part typed by an item def", src: `item def A;
abstract item def B {
	public abstract part a : A;
}`},
		{name: "part subsetting an item usage typed by an item def", src: `item def I;
item i : I;
part f :> i;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if errs := w8cLibraryErrorsIn(t, "<t>.sysml", tc.src); len(errs) != 0 {
				t.Errorf("errors = %v, want none", errs)
			}
		})
	}
}
