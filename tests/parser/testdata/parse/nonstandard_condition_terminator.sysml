// A bare condition is a body's result expression (SysML v2 8.2.2.19
// ResultExpressionMember), which no `;` follows; a written `;` is kept on the
// member as its terminator.
package ConditionTerminators {
    private import ScalarValues::Real;
    constraint def Positive { in x : Real; x > 0.0; }
    constraint def Bounded { in x : Real; x > 0.0 }
    constraint positive { attribute m : Real; m > 0.0; }
    constraint def Two { in x : Real; x > 0.0; x < 10.0 }
    part def P {
        attribute m : Real;
        assert constraint { m > 0.0; }
        assert constraint c : Positive;
    }
}
