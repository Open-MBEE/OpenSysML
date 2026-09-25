// The model the interop harness round-trips through a running Flexo stack.
// Structure only, so the whole model converts to Turtle, and deliberately
// shaped to reach every property class the round trip can lose: a nesting
// hierarchy, a multi-valued specialization list, expression-valued positions,
// an unnamed member, a short name, a flag, and a connector.
package Interop {
    doc /* A rover and its ground link. */

    part def Component {
        attribute mass = 0.0;
    }

    part def Powered;

    part def <bat> Battery specializes Component, Powered {
        attribute capacity = 1500.0;
    }

    abstract part def Wheel specializes Component {
        attribute diameter = 0.25;
    }

    port def TelemetryPort;

    part def Rover specializes Component {
        part battery : Battery;
        part wheels : Wheel[6];
        port telemetry : TelemetryPort;
    }

    part def GroundStation {
        port uplink : TelemetryPort;
    }

    part def Mission {
        part rover : Rover;
        part station : GroundStation;

        connect rover.telemetry to station.uplink;
    }

    comment about Wheel /* Diameter is in metres. */
}
