// A value written under either name of a redefined feature is the value both names read (KerML 1.1 §7.3.4.5);
// a name the redefinition removes (§8.3.3.1.10) is written qualified and read through a typed reference.
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
        ref onBaseAsBand : Band = onBase;
        attribute baseRedefining : Real = onBaseAsBand.bandCost;
        ref renamedTwiceAsDeluxe : Deluxe = renamedTwice;
        attribute deep : Real = renamedTwiceAsDeluxe.deluxeCost;
        attribute throughFamily : Real = configured.band.bandCost;
    }
}
