// Invalid: a function body ends in at most one result expression; a second bare expression is a second result expression (KerML.xtext:933 FunctionBodyPart, KerML 8.3.4.8 Function::result).
package K20FunctionBodySecondResultExpression {
    private import ScalarValues::Real;
    function twice {
        in x : Real;
        x + 1.0
        x + 2.0
    }
}
