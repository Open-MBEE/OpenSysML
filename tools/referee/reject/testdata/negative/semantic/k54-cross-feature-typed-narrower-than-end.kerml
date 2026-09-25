// Invalid: a cross feature declared ahead of its end's kind keyword must have the same type as the end, not a subtype of it (KerML 8.3.3.3 validateFeatureCrossFeatureType; pilot validateFeatureCrossFeatureType).
package K54CrossFeatureTypedNarrowerThanEnd {
    class C1; class C2;
    class Sub1 :> C1;
    feature g : C1;
    assoc A {
        end x1 : Sub1 [0..1] :> g feature x : C1;
        end y1 [0..1] feature y : C2;
    }
}
