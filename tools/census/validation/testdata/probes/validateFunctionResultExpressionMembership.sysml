// Census: validateFunctionResultExpressionMembership
// Expect: error: Only one (owned or inherited) result expression is allowed
package P {
    private import ScalarValues::Real;
    constraint def Base { in x : Real; x > 0.0 }
    constraint def Sub :> Base { x > 1.0 }
}
