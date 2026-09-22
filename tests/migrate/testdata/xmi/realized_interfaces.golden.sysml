package Bus {
    port def Telemetry {
        abstract action def Report;
        ref action report : Report;
    }
    port def Command;
}
part def Sensor {
    port telemetry : Bus::Telemetry;
}
part def Controller {
    port command : Bus::Command;
    port diagnostics : Diagnostics;
    port def Diagnostics;
    port telemetry : Bus::Telemetry;
}
port def FastTelemetry :> Bus::Telemetry;
constraint def Budget {
    /* not migrated: InterfaceRealization (_budgetCmd) — a constraint def cannot specialize the port def the interface Bus::Command becomes */
}
part def Gateway {
    /* not migrated: InterfaceRealization (_gwLost) — the realized interface is not in the document; 1 contract reference(s) resolve to nothing in the document (_lost) */
    /* not migrated: InterfaceRealization (_gwLib) — the realized interface http://www.example.org/libraries/Comms.xmi#_remoteIf is library content, which is not written */
}
