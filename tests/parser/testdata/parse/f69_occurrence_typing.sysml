// A usage's typing target is settled semantically, not syntactically: a part
// typed by an item def, a nested use case, a bind to a subtype-typed value and
// a typed succession all parse through UsageDeclaration (SysML.xtext:730).
package F69OccurrenceTyping {
    item def Box;
    part carried : Box;

    use case def Drive;
    part def Vehicle {
        use case drive : Drive;
    }

    part def Truck :> Vehicle;
    part v : Vehicle;
    part t : Truck = v;

    connection def Before;
    action def Process {
        action paint;
        action dry;
        succession s : Before first paint then dry;
    }
}
