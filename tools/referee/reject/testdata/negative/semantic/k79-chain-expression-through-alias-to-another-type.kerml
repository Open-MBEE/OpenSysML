// Invalid: the target feature of a feature chain expression must be featured within the result of its argument, and an alias or import reaching another type's feature is not (KerML 8.3.4.8.4 validateFeatureChainExpressionConformance; pilot validateFeatureChainExpressionFeatureConformance).
package K79ChainExpressionThroughAliasToAnotherType {
    class B { feature y : B; }
    class A { alias z for B::y; feature w : B; public import B::y; }
    feature k : A;
    feature r1 = k.z;
    feature r2 = k.y;
    feature r3 = k.w.y;
    class K {
        feature kk : A;
        feature s1 = kk.z;
        feature s2 = kk.w.y;
    }
}
