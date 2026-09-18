// Invalid: Nested usages in a port usage (other than ports) must be referential (SysML 7.12 Ports; pilot validatePortUsageNestedUsagesNotComposite).
package S25PortNestedCompositePart {
    part def B;
    port def PD;
    part p {
        port pt : PD {
            part b : B;
        }
    }
}
