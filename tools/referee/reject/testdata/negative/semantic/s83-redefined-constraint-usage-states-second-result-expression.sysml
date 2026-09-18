// Invalid: a redefining constraint usage inherits the redefined constraint's result expression and may not state a second (KerML 8.3.4.8 validateExpressionResultExpressionMembership; pilot validateExpressionResultExpressionMembership).
package S53RedefinedConstraintUsageStatesSecondResultExpression {
    private import ScalarValues::Real;
    requirement def Margin {
        attribute x : Real;
        require constraint enough { x > 0.0 }
    }
    requirement def Tight :> Margin {
        require constraint :>> enough { x > 1.0 }
    }
}
