// Invalid: a calculation body ends in at most one result expression; a second bare expression is a second result expression (SysML.xtext:1951 CalculationBodyPart, KerML 8.3.4.8 Function::result).
package G69CalcBodySecondResultExpression {
    private import ScalarValues::Real;
    calc def Twice {
        in x : Real;
        x + 1.0
        x + 2.0
    }
}
