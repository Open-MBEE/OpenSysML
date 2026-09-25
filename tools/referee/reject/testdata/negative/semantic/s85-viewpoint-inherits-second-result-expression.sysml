// Invalid: a viewpoint is a requirement and so a constraint, and may not inherit result expressions from two generals (KerML 8.3.4.8 validateFunctionResultExpressionMembership; pilot validateFunctionResultExpressionMembership).
package S55ViewpointInheritsSecondResultExpression {
    private import ScalarValues::Real;
    attribute x : Real;
    constraint def A { x > 0.0 }
    constraint def B { x < 9.0 }
    viewpoint def V :> A, B;
}
