// Invalid: a redefinition declaring no direction does not conform to a redefined `in`, `out` or `inout` feature (KerML 8.3.3.3.8 validateRedefinitionDirectionConformance; pilot validateRedefinitionDirectionConformance).
package K76UndirectedRedefinitionOfDirectedFeature {
    class C { in x; inout y; }
    class D specializes C { feature :>> x; feature :>> y; }
}
