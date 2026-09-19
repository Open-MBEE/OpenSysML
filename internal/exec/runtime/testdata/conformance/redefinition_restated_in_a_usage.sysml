// A redefinition names one feature, so `mass` answers what a usage restated
// `own` to rather than the definition's default (KerML 1.0 §7.3.4.5).
package test {
    private import ScalarValues::Real;
    private import RealFunctions::*;

    part def Comp { attribute mass : Real = 0.0; }

    part def Sys :> Comp {
        attribute own :>> mass = 1.5;
        part subs : Comp[*];
        attribute total : Real = own + sum(subs.mass);
    }

    part sat : Sys {
        attribute :>> own = 10.0;
        part payload : Comp :> subs { attribute :>> mass = 2.0; }
    }
}
