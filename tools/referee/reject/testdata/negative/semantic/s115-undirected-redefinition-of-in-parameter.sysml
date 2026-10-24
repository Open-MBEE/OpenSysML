// Invalid: a redefinition of an `in` parameter that declares no direction does not conform to it (KerML 8.3.3.3.8 validateRedefinitionDirectionConformance; pilot validateRedefinitionDirectionConformance).
package S115UndirectedRedefinitionOfInParameter {
    private import ScalarValues::Real;
    action def Burn { in durationS : Real; }
    action burn : Burn { :>> durationS = 3.0; }
}
