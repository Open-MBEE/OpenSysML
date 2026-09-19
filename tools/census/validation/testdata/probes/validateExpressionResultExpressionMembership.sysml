// Census: validateExpressionResultExpressionMembership
// Expect: error: Only one (owned or inherited) result expression is allowed
package P {
    private import ScalarValues::Real;
    requirement def Margin {
        attribute x : Real;
        require constraint enough { x > 0.0 }
    }
    requirement def Tight :> Margin {
        require constraint :>> enough { x > 1.0 }
    }
}
