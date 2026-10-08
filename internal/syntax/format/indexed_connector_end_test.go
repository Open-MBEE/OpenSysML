package format

import "testing"

// An indexed connector end (`s.y#(1)`, an OpenSysML extension) is written back
// as it was read, in every form that admits it, and the result is stable.
func TestIndexedConnectorEndsAreWrittenBack(t *testing.T) {
	src := `package F3Idx {
    part def Asm {
        part s : Source;
        part k : Sink;
        attribute i : Integer = 2;
        connection : C connect [1] s.y#(1) to [1] k.u;
        connect s.y#(i + 1) to k.u#(1);
        connect (s.y#(1), k.u, s.y#(2));
        interface s.y#(1) to k.u;
        allocate s.y#(1) to k.u;
        bind s.y#(2) = k.u;
        flow s.y#(1) to k.u;
        flow f from s.y#(i) to k.u;
    }
}
`
	got := format(t, src)
	if got != src {
		t.Fatalf("formatting changed the indexed ends\n--- want ---\n%s\n--- got ---\n%s", src, got)
	}
	checkStable(t, "indexed ends", []byte(got))
}
