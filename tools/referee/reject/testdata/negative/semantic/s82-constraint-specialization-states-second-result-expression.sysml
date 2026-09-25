// Invalid: a constraint definition specializing one that owns a result expression may not state a second (KerML 8.3.4.8 validateFunctionResultExpressionMembership; pilot validateFunctionResultExpressionMembership).
package S52ConstraintSpecializationStatesSecondResultExpression {
    private import ScalarValues::Real;
    constraint def Base { in x : Real; x > 0.0 }
    constraint def Sub :> Base { x > 1.0 }
}
