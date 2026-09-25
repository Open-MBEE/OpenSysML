// Invalid: an end that declares its cross feature inline (`end [m] ...`) cannot also cross another feature (KerML 8.3.3.3 Feature::crossFeature is single-valued; pilot checkFeatureCrossingSpecialization).
package K43InlineCrossFeatureAlsoCrosses {
    class C1 {
        feature a : C2;
    }
    class C2 {
        feature b : C1;
    }
    assoc A1 {
        end [1] feature x : C1 crosses y.b;
        end feature y : C2 crosses x.a;
    }
}
