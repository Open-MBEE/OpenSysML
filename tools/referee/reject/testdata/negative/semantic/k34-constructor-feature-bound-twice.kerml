// Invalid: a constructor expression cannot bind the same feature twice (KerML 8.3.4.8 validateConstructorExpressionNoDuplicateFeatureRedefinition; pilot validateConstructorExpressionNoDuplicateFeatureRedefinition).
package K34ConstructorFeatureBoundTwice {
    class T;
    class C { feature x : T; }
    feature t : T;
    feature c : C = new C(x = t, x = t);
}
