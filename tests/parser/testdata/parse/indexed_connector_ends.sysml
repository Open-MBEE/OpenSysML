// Indexed connector ends: `#( index )` selects one element of a
// collection-valued feature as the end (an OpenSysML extension).
package IndexedConnectorEnds {
    private import ScalarValues::*;
    port def P { attribute value : Real default 0.0; }
    part def Source { port y : P[2]; }
    part def Sink { port u : P; }
    connection def C { end source[1] : P; end target[1] : P; }
    attribute i : Integer = 2;
    part def Asm {
        part s : Source;
        part k : Sink;
        connection : C connect [1] s.y#(1) to [1] k.u;
        connect s.y#(i) to k.u;
        connect (s.y#(1), k.u, s.y#(2));
        interface s.y#(1) to k.u;
        allocate s.y#(1) to k.u;
        bind k.u = s.y#(2);
        flow from s.y#(1) to k.u;
        flow of Real from s.y#(i + 0) to k.u;
    }
}
