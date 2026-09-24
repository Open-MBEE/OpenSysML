// Invalid: Featuring types of redefining feature and redefined feature cannot be the same (KerML 7.4.7 validateRedefinitionFeaturingTypes; pilot Redefinition_OwningType_Invalid.sysml.xt:38).
package P32RedefinitionSameFeaturingType {
    part def Engine;
    part def B {
        alias eng1 for Vehicle::eng;
    }
    part def Vehicle specializes B {
        part eng : Engine;
        part smallEng : Engine redefines eng1;
    }
}
