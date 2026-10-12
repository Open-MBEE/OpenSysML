// Invalid: a connector end is a feature chain, so `#( index )` after it has no production — the index operator belongs to expressions (SysML.xtext ConnectorEndMember, ConnectorEnd; KerML.xtext IndexExpression).
package X11IndexedConnectorEnd {
    private import ScalarValues::*;
    port def P { attribute value : Real default 0.0; }
    part def Source { port y : P[2]; }
    part def Sink { port u : P; }
    connection def C { end source[1] : P; end target[1] : P; }
    part def Asm {
        part s : Source;
        part k : Sink;
        connection : C connect [1] s.y#(1) to [1] k.u;
    }
}
