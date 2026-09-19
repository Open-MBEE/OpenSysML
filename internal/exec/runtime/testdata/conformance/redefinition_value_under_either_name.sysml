// A redefinition renames one feature (KerML 1.0 §7.3.4.5), so a value written
// under either name is the value both names read, however many names the chain
// of redefinitions gives it and whichever declaration wrote it. The redefinition
// hides the base name from its owner's members, so writing under that name
// qualifies it and reading under it goes through a base-typed reference.
package test {
    private import ScalarValues::Real;

    part def Ring { attribute ringCost : Real; attribute color : Real default = 1.0; }
    part def Band :> Ring { attribute bandCost :>> ringCost; }
    part def Deluxe :> Band { attribute deluxeCost :>> bandCost; }

    part onRedefining : Band { attribute :>> bandCost = 400.0; }
    part onBase : Band { attribute :>> Ring::ringCost = 500.0; }
    part renamedTwice : Deluxe { attribute :>> Ring::ringCost = 700.0; }

    // The value is written in an abstract part usage the configured one
    // specializes, so it reaches the redefinition through that usage.
    abstract part family { part band : Band { attribute :>> bandCost = 400.0; } }
    part configured :> family { part :>> band { attribute :>> color = 2.0; } }

    part reader {
        attribute redefiningName : Real = onRedefining.bandCost;
        ref onRedefiningAsRing : Ring = onRedefining;
        attribute redefiningBase : Real = onRedefiningAsRing.ringCost;
        ref onBaseAsRing : Ring = onBase;
        attribute baseName : Real = onBaseAsRing.ringCost;
        attribute baseRedefining : Real = onBase.bandCost;
        attribute deep : Real = renamedTwice.deluxeCost;
        attribute throughFamily : Real = configured.band.bandCost;
    }
}
