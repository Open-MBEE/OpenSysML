// Invalid: a constraint usage reference-subsetting one that owns a result expression inherits it and may not state a second (KerML 8.3.4.8 validateExpressionResultExpressionMembership; pilot validateExpressionResultExpressionMembership).
package S54ReferencingConstraintUsageStatesSecondResultExpression {
    private import ScalarValues::Real;
    attribute x : Real;
    constraint c { x > 0.0 }
    constraint d ::> c { x > 1.0 }
}
