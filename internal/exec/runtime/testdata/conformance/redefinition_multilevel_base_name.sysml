// A chain of redefinitions names one feature at every level, so an expression
// reading a base name sees the value written under the innermost name.
package test {
    private import ScalarValues::Real;

    part def Comp { attribute mass : Real = 0.0; }
    part def Sys :> Comp { attribute own :>> mass = 1.5; }
    part def Bus :> Sys { attribute dry :>> own = 4.0; }

    part bus : Bus { attribute :>> dry = 7.0; }
    part inherited : Bus;

    part reader {
        attribute busDry : Real = bus.dry;
        attribute busOwn : Real = bus.own;
        attribute busMass : Real = bus.mass;
        attribute inheritedMass : Real = inherited.mass;
        attribute rollUp : Real = bus.mass + inherited.mass;
    }
}
