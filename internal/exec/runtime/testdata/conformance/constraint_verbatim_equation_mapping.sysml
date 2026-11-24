// A constraint body whose annotations carry Modelica equations verbatim. The
// three `body` Strings name a medium state function, a stream value and an
// event-regularising conditional that have no SysML expression form; they stay
// text. The body's own condition is an ordinary SysML expression and decides
// the constraint.
package test {
    private import ScalarValues::*;

    metadata def EquationMapping {
        attribute body : String;
    }

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
