// A chain of redefinitions names one feature at every level, so a base name
// read through a reference typed by the level declaring it sees the value
// written under the innermost name. Each redefinition removes the name it
// redefines from its owner's namespace (KerML 1.0 §7.3.4.5), so `bus.own` and
// `bus.mass` resolve nowhere; the base-typed reference is how the name is read.
package test {
    private import ScalarValues::Real;

    part def Comp { attribute mass : Real = 0.0; }
    part def Sys :> Comp { attribute own :>> mass = 1.5; }
    part def Bus :> Sys { attribute dry :>> own = 4.0; }

    part bus : Bus { attribute :>> dry = 7.0; }
    part inherited : Bus;

    part reader {
        attribute busDry : Real = bus.dry;
        ref busAsSys : Sys = bus;
        attribute busOwn : Real = busAsSys.own;
        ref busAsComp : Comp = bus;
        attribute busMass : Real = busAsComp.mass;
        ref inheritedAsComp : Comp = inherited;
        attribute inheritedMass : Real = inheritedAsComp.mass;
        attribute rollUp : Real = busAsComp.mass + inheritedAsComp.mass;
    }
}
