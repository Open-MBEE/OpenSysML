// Invalid: an association end must have exactly one type (KerML 8.2.4.6.2 checkAssociationEndTypes; pilot validateAssociationEndTypes, AssociationTest_EndTypes_invalid.kerml.xt).
package P02AssociationEndTwoTypes {
    class C1;
    class C2;
    assoc A {
        end x : C1;
        end y : C1, C2;
    }
}
