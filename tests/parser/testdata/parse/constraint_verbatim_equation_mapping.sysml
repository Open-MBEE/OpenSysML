// A verbatim equation mapping: an annotation whose `body` String carries a
// Modelica equation exactly as written. The engine does not interpret the body
// as a SysML expression; it carries it unchanged through parse, evaluation and
// the api-json / RDF element form.
package ModelicaMapping {
    private import ScalarValues::*;

    metadata def EquationMapping {
        attribute body : String;
    }
}

package Steam {
    private import ScalarValues::*;
    private import ModelicaMapping::*;

    part def SteamGenerator {
        attribute state_a;
        attribute m_flow : Real = 2.0;
        port port_a;
        port port_b;

        constraint steamEquations {
            @EquationMapping { body = "Medium.temperature(state_a)"; }
            @EquationMapping { body = "inStream(port_a.h_outflow)"; }
            @EquationMapping { body = "noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)"; }
            m_flow > 0.0
        }
    }
}
