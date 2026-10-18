// Invalid: a usage subsets a Feature, so a part subsetting an attribute definition names nothing that position admits (KerML 8.3.3.3 Subsetting::subsettedFeature is typed Feature, a rule SysML usages inherit; pilot fails to link the Subsetting — `Couldn't resolve reference to Feature 'Mass'` — exactly as for KerML's `feature f :> D`).
package S102PartSubsetsAttributeDef {
    private import ScalarValues::*;
    part def Engine;
    attribute def Mass;
    part eng : Engine :> Mass;
}
