// A `first` end reached through a feature chain is a succession-as-usage
// (KerML.xtext:699 OwnedReferenceSubsetting), not an initial node whose member
// element is a plain name (SysML.xtext:1384 InitialNodeMember).
package ChainedSuccessionEnds {
    part def Vehicle {
        action doorClosed;
    }
    part def Driver {
        action driverReady;
    }
    action def Startup {
        part vehicle : Vehicle;
        part driver : Driver;
        first vehicle.doorClosed then driver.driverReady;
    }
    state def Cycle {
        part vehicle : Vehicle;
        state off;
        state on;
        first off then on;
    }
    // A plain `first` end still declares an initial node.
    action def Plain {
        action start;
        action target;
        first start then target;
    }
}
