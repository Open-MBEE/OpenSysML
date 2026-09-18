// Invalid: A port must be typed by port definitions (SysML 7.12 Ports; pilot validatePortUsageType).
package S26PortTypedByPartDef {
    part def PD;
    part p {
        port pt : PD;
    }
}
