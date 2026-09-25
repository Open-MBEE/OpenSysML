// Invalid: each chaining feature of a feature chain must be featured within the one before it, and an alias reaching another type's feature is not (KerML 8.3.3.3.4 validateFeatureChainingFeatureConformance; pilot validateFeatureChainingFeatureConformance).
package K47FeatureChainThroughAliasToAnotherType {
    class B { feature y : B; }
    class A { alias z for B::y; feature w : B; }
    class K {
        feature k : A;
        feature q : B;
        feature r1 references k.z;
        feature r2 chains k.z;
        feature r3 :> k.z;
        connector c from k.z to q;
    }
}
