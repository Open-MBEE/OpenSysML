// A lander and the views that present it, one per rendering kind: see
// VIEWS-DEMO.md for the walkthrough.
package Lander {
    private import ScalarValues::*;

    metadata def Safety;

    port def FuelPort;

    port def TelemetryPort;

    item def Fuel;

    attribute def Signal;

    #Safety part def Thruster {
        attribute thrust : Real = 4500.0;
        in item fuelIn : Fuel;
        port supply : FuelPort;
    }

    part def Tank {
        attribute capacity : Real = 320.0;
        out item fuelOut : Fuel;
        port outlet : FuelPort;
    }

    #Safety part def Parachute;

    part def Antenna {
        port downlink : TelemetryPort;
    }

    connection def FuelLine {
        end supplier : FuelPort;
        end consumer : FuelPort;
    }

    part def Descender {
        attribute mass : Real;
        attribute partCount : Integer = 4;

        part tank : Tank;
        part thruster : Thruster;
        part parachute : Parachute;
        part antenna : Antenna;

        connection supply : FuelLine connect tank.outlet to thruster.supply;
        flow of Fuel from tank.fuelOut to thruster.fuelIn;
    }

    // A machine a state view presents, states and transitions alike.
    state def DescentStates {
        attribute altitude : Real = 9000.0;

        entry; then cruise;

        state cruise;
        state descent {
            entry; then braking;

            state braking;
            state hover;

            transition braking_to_hover
                first braking
                accept Signal
                if altitude < 100.0
                then hover;
        }
        state landed;

        transition first cruise then descent;
        transition first descent then landed;
    }

    // A behavior an action view presents, nodes and successions alike.
    action def Descend {
        attribute altitude : Real = 9000.0;

        in item fuel : Fuel;

        first start;

        action deployParachute;
        action burn {
            in item propellant : Fuel;
            action ignite;
            action throttle;
            first ignite;
            succession first ignite then throttle;
        }

        fork split;
        join sync;
        action touchdown;

        succession first start then split;
        succession first split then deployParachute;
        succession first split then burn;
        succession first deployParachute then sync;
        succession first burn then sync;
        succession first sync then check;

        decide check;
        if altitude > 0.0 then touchdown;
        else done;
        succession first touchdown then done;
    }

    // Two landers a concern's condition is checked against: one within the mass
    // budget and one over it.
    part descender : Descender {
        attribute :>> mass = 890.0;
    }

    part heavyDescender : Descender {
        attribute :>> mass = 1180.0;
    }

    concern def MassBudget {
        subject lander : Descender;
        attribute maxMass = 1000.0;
        require constraint {
            lander.mass <= maxMass
        }
    }
}

package LanderViews {
    private import Views::*;
    private import StandardViewDefinitions::*;

    viewpoint def MassPerspective {
        frame concern mass : Lander::MassBudget;
    }

    viewpoint massPerspective : MassPerspective;

    view interfaces {
        expose Lander::Descender;
        render asInterconnectionDiagram;
    }

    view partsTable {
        expose Lander::Descender;
        render asElementTable;
    }

    // The rendering kind comes from the standard view definition each of these
    // specializes, with no rendering member of their own.
    view descentStates : StateTransitionView {
        expose Lander::DescentStates;
    }

    view descentFlow : ActionFlowView {
        expose Lander::Descend;
    }

    // A filtered recursive expose, which reaches only the safety-critical
    // elements of the model.
    view safetyView {
        expose Lander::**[@Lander::Safety];
    }

    // States no rendering, so a containment tree renders it, with the
    // interconnection view nested as a subtree.
    view overview {
        expose Lander::descender;
        expose Lander::heavyDescender;
        view interfaceSubview :> interfaces;

        // `%view` checks the exposed landers against the concern the
        // satisfied viewpoint frames.
        satisfy massPerspective;
    }
}
