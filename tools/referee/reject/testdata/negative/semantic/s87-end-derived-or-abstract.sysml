// Invalid: an end feature cannot be derived, abstract, composite or portion (KerML 8.3.3.3.4 validateFeatureEndNotDerivedAbstractCompositeOrPortion; pilot validateFeatureEndNotDerivedAbstractCompositeOrPortion).
package S53EndDerivedOrAbstract {
    part def A;
    connection def K1 { end derived a : A; end b : A; }
    connection def K2 { end abstract a : A; end b : A; }
    connection def K3 { end variation a : A; end b : A; }
}
