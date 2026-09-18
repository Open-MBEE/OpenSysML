// Invalid: a cross feature declared ahead of its end's kind keyword must have the same type as the end, not a subtype of it (KerML 8.3.3.3 validateFeatureCrossFeatureType; pilot validateFeatureCrossFeatureType).
package S88CrossFeatureTypedNarrowerThanEnd {
    part def A; part def B;
    part def Sub1 :> A;
    item g : A;
    connection def Conn {
        end x1 : Sub1 [0..1] :> g item x : A;
        end y1 [0..1] item y : B;
    }
}
