// Reading verbatim equations back through `.metadata#(i).body`, in the
// order the annotations were stated.
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

    part def Report {
        attribute eqOne = SteamGenerator::steamEquations.metadata#(1).body;
        attribute eqTwo = SteamGenerator::steamEquations.metadata#(2).body;
        attribute eqThree = SteamGenerator::steamEquations.metadata#(3).body;
    }
}
