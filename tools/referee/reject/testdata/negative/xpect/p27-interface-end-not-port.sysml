// Invalid: An interface end must be a port (SysML 7.13 validateInterfaceUsageEnd; pilot InterfaceUsage_Invalid.sysml.xt:63).
package P27InterfaceEndNotPort {
    part def Fuel;
    port def FuelInPort;
    part vehicle {
        part fuel : Fuel;
        port inPort : FuelInPort;
        interface bad {
            end part ::> fuel;
            end port ::> inPort;
        }
    }
}
