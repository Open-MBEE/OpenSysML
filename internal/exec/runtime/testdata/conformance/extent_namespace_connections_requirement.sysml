// The reported model: a requirement counting `all Link` is satisfied only when the
// package-level `outer` counts beside the nested `inner` (SysML v2.1 Beta 2 §8.4.9.2:
// a ConnectionUsage is a PartUsage; KerML 1.1 Beta 2 §7.4.9.2: `all T` is every instance
// of T).
package Q {
    private import SequenceFunctions::*;
    connection def Link { end a; end b; }
    part def Thing;
    part q : Thing;
    part sys {
        part x; part y;
        connection inner : Link connect x to y;
    }
    connection outer : Link connect sys.x to sys.y;
    requirement r { subject s = sys; require constraint { size(all Link) == 2 } }
}
