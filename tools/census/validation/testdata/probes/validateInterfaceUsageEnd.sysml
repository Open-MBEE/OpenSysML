// Census: validateInterfaceUsageEnd
// Expect: error: An interface end must be a port.
package P {
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
