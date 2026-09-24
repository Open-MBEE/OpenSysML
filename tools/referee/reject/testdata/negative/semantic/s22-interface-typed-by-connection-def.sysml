// Invalid: An interface must be typed by interface definitions (SysML 7.14 Interfaces; pilot validateInterfaceUsageType).
package S22InterfaceTypedByConnectionDef {
    port def PT;
    connection def CD {
        end port a : PT;
        end port b : PT;
    }
    part p {
        port p1 : PT;
        port p2 : PT;
        interface i : CD connect p1 to p2;
    }
}
