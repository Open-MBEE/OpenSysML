// The Connector metaclass family read by specification name: a binding
// connector and a connection between two parts, their ends and related
// elements, and the reason a connector end is not answered.
package ConnectorFamily {
    private import ScalarValues::*;
    port def Plug { attribute volts : Real; }
    part def Charger { port outlet : Plug; }
    part def Battery { port inlet : Plug; }
    part def Station {
        part charger : Charger;
        part battery : Battery;
        connect charger.outlet to battery.inlet;
        attribute level : Real;
        attribute shown : Real;
        bind level = shown;
    }
}
