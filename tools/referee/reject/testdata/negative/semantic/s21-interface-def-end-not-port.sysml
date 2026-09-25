// Invalid: An interface definition end must be a port (SysML 7.14 Interfaces; pilot validateInterfaceDefinitionEnd).
package S21InterfaceDefEndNotPort {
    part def P;
    port def PT;
    interface def ID {
        end part a : P;
        end port b : PT;
        end port c : PT;
    }
}
