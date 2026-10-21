// KerML function and predicate bodies end in a ResultExpressionMember too
// (KerML 8.2.5.7.1 FunctionBodyPart), so a `;` after an invariant's or a
// predicate's condition is kept as the member's terminator.
package ConditionTerminators {
    private import ScalarValues::Real;
    predicate Positive { in x : Real; x > 0.0; }
    predicate Bounded { in x : Real; x > 0.0 }
    classifier C {
        feature m : Real;
        inv positive { m > 0.0; }
        inv bounded { m < 10.0 }
    }
}
