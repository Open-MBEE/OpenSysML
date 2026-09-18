// Invalid: A connection must be typed by connection definitions (SysML 7.13 Connections; pilot validateConnectionUsageType).
package S15ConnectionTypedByPartDef {
    part def P;
    part def PD {
        end part x : P;
        end part y : P;
    }
    part a : P;
    part b : P;
    connection c : PD connect a to b;
}
