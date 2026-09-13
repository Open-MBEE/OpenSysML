// A valued quantity attribute still derives what its value type declares: the
// dimensions its measurement reference states, and the unit read from the value.
package test {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;

    part def Engine {
        attribute mass :> ISQ::mass;
    }

    part def Rocket {
        part engine : Engine {
            :>> mass = 100 [kg];
        }
        attribute dimensions : Positive[0..*] = engine.mass.dimensions;
        attribute unit : MassUnit = engine.mass.mRef;
    }
}
