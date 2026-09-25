// Invalid: a redefinition owned by a feature of a type does not see an alias the owning type itself declares (KerML 8.2.3.5.2 redefinition name resolution; pilot `Couldn't resolve reference to Feature 'y'`).
package S96RedefinitionNamesOwningTypeAlias {
    private import ScalarValues::Real;
    part def A {
        attribute x : Real;
    }
    part def B :> A {
        alias y for x;
        attribute z :>> y;
    }
}
