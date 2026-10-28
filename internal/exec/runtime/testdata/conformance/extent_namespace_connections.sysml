// A connection usage a package declares denotes an instance of its connection definition
// (SysML v2.1 Beta 2 §8.4.9.2: a ConnectionUsage is a PartUsage), so `all Link` reaches it
// as it reaches the nested `inner`, and the usage reads as that one object with its ends
// attached to the features the `connect` clause names — exactly as a nested connection
// does. Interface and allocation usages are connection usages (§8.4.10.2, §8.4.13.2) and
// follow. A stated multiplicity governs as it does for a part: `[0..*]` yields no object,
// `[2]` two connectors each attached, a usage stating none the one representative object;
// two usages a `bind` joins share one connector with its ends.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    connection def Link { end a; end b; }
    connection def Tie { end a; end b; }
    port def Pt;
    interface def Ifc { end port x : Pt; end port y : Pt; }
    allocation def Aloc { end part x; end part y; }
    part def Thing;

    part q : Thing;
    part sys {
        part x; part y;
        port px : Pt; port py : Pt;
        connection inner : Link connect x to y;
    }
    connection outer : Link connect sys.x to sys.y;
    connection none : Link[0..*] connect sys.x to sys.y;
    connection pair : Link[2] connect sys.x to sys.y;
    connection c1 : Link connect sys.x to sys.y;
    connection c2 : Link connect sys.x to sys.y;
    bind c1 = c2;
    connection lone : Tie connect sys.x to sys.y;
    interface wire : Ifc connect sys.px to sys.py;
    allocation alloc : Aloc allocate sys.x to sys.y;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Thing) == 1,
            size(all Link) == 5,
            size(all Tie) == 1,
            size(all Ifc) == 1,
            size(all Aloc) == 1,
            size(outer) == 1,
            outer == outer,
            outer != sys.inner,
            outer.a === sys.x,
            outer.b === sys.y,
            sys.inner.a === sys.x,
            size(none) == 0,
            size(pair) == 2,
            pair#(1) != pair#(2),
            pair#(2).a === sys.x,
            c1 == c2,
            c1.a === sys.x,
            c2.b === sys.y,
            wire.x === sys.px,
            alloc.y === sys.y
        );
    }
}
