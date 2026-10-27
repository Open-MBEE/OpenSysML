// A connection usage nested in a part follows the same multiplicity rule as one a package
// declares: a connector is a feature whose values are links (KerML 1.1 Beta 2 §7.4.6.1) and a
// multiplicity bounds how many values it holds (§7.4.12), a connection usage takes no implicit
// `[1..1]` (SysML v2.1 Beta 2 §7.6.3), and each end holds one participant (§7.13.2). So `[2]`
// yields two Links each with its ends attached to the features the `connect` clause names,
// `[0..*]` none, `[1..3]` its lower bound, and `all Link` reaches every one of them. Interface
// and allocation usages are connection usages (§8.4.10.2, §8.4.13.2) and follow.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    connection def Link { end a; end b; }
    connection def Tie { end a; end b; }
    port def Pt;
    interface def Ifc { end port x : Pt; end port y : Pt; }
    allocation def Aloc { end part x; end part y; }

    part sys {
        part x; part y;
        port px : Pt; port py : Pt;
        connection one : Link connect x to y;
        connection pair : Link[2] connect x to y;
        connection none : Link[0..*] connect x to y;
        connection some : Tie[1..3] connect x to y;
        interface wires : Ifc[2] connect px to py;
        allocation allocs : Aloc[2] allocate x to y;
    }

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Link) == 3,
            size(all Tie) == 1,
            size(all Ifc) == 2,
            size(all Aloc) == 2,
            size(sys.pair) == 2,
            sys.pair#(1) != sys.pair#(2),
            sys.pair#(1) != sys.one,
            sys.pair#(1).a === sys.x,
            sys.pair#(2).a === sys.x,
            sys.pair#(2).b === sys.y,
            size(sys.pair.a) == 2,
            size(sys.none) == 0,
            size(sys.some) == 1,
            sys.some#(1).b === sys.y,
            sys.wires#(2).x === sys.px,
            sys.allocs#(1).y === sys.y
        );
    }
}
