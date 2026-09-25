// One declaration values the same feature under both of its names, which
// redefinition makes one feature: neither value may be picked silently. The
// redefinition hides the base name, so that name is written qualified.
package test {
    private import ScalarValues::Real;

    part def Ring { attribute ringCost : Real; }
    part def Band :> Ring { attribute bandCost :>> ringCost; }

    part conflicted : Band {
        attribute :>> bandCost = 400.0;
        attribute :>> Ring::ringCost = 500.0;
    }
}
