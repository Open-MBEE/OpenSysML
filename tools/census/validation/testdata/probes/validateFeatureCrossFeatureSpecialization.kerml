// Census: validateFeatureCrossFeatureSpecialization
// Expect: error: Cross feature must specialized redefined-end cross features
package AssociationTest {
    class C1 { feature a : C2; }
    class C2 { feature b : C1; feature c subsets b; }
    assoc A1 {
        end x : C1 crosses y.b;
        end y : C2 crosses y.b;
    }
    assoc A2 specializes A1 {
        end x : C1 crosses y.c;
        end y : C2 crosses x.a;
    }
}
