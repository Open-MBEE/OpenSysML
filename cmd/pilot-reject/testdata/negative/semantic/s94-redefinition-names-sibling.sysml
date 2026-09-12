// Invalid: a redefinition owned by a feature of a type names a feature of that type's generals or of the enclosing namespaces, never a sibling member of the owning type (KerML 8.2.3.5.2 redefinition name resolution; pilot `Couldn't resolve reference to Feature 'x'`).
package S94RedefinitionNamesSibling {
    private import ScalarValues::Real;
    part def B {
        attribute x : Real;
        attribute z :>> x;
    }
}
