// The team around the robot of disposal-robot-demo: the truck that carries it,
// the console that commands it, and the callout they answer.
package Team {
    private import ScalarValues::*;
    private import SI::*;
    private import ISQ::*;
    private import SequenceFunctions::*;
    private import RealFunctions::sum;
    private import ControlFunctions::*;

    // Quantities carry units, so the payload budget is stated in kilograms.
    part def Robot {
        attribute mass : MassValue = 340.0 [kg];
        attribute reach : LengthValue = 2.1 [m];
        attribute charge : Real = 1200.0;
    }

    part def Cradle {
        attribute mass : MassValue = 60.0 [kg];
    }

    part def Truck {
        attribute curbMass : MassValue = 4200.0 [kg];
        attribute payloadLimit : MassValue = 1200.0 [kg];

        part cradles : Cradle[2];
        part robots : Robot[2];

        attribute payload : MassValue = sum(robots.mass) + sum(cradles.mass);

        assert constraint withinPayload {
            payload <= payloadLimit
        }
    }

    // Sequence operations ask about the fleet without naming its members.
    calc def FleetReach {
        in robots : Robot[*];
        in floor : LengthValue;

        return attribute usable : LengthValue[*] nonunique =
            robots->select { in r : Robot; r.reach >= floor }
                  ->collect { in r : Robot; r.reach };
    }

    calc def Endurance {
        in charges : Real[*] nonunique;
        in draw : Real;

        return attribute minutes : Real =
            (charges->reduce { in a : Real; in b : Real; a + b }) / draw;
    }
}

// The console commands the unit over an interface of conjugated ports: one
// message, delivered across the connector their site declares.
package TeamComms {
    private import ScalarValues::*;

    item def Command {
        attribute code : Integer;
    }

    port def CommandPort {
        out item issued : Command;
    }

    interface def CommandLink {
        end commanding : CommandPort;
        end commanded : ~CommandPort;
    }

    part def Console {
        port command : CommandPort;
        attribute issued : Integer = 0;

        item approach : Command { attribute :>> code = 1; }
        item disarm : Command { attribute :>> code = 2; }

        // The operator works the console: each state sends the next order.
        exhibit state dispatch {
            entry; then ready;

            state ready;

            state approaching {
                entry action orderApproach {
                    send approach via command;
                    assign issued := issued + 1;
                }
            }

            state disarming {
                entry action orderDisarm {
                    send disarm via command;
                    assign issued := issued + 1;
                }
            }

            succession first ready then approaching;
            succession first approaching then disarming;
        }
    }

    part def Unit {
        port command : ~CommandPort;
        attribute accepted : Integer = 0;
        attribute lastCode : Integer = 0;

        exhibit state duty {
            entry; then standingBy;

            state standingBy;

            state working {
                entry action count {
                    assign accepted := accepted + 1;
                }
            }

            transition first standingBy
                accept order : Command via command
                do assign lastCode := order.code
                then working;

            transition first working
                then standingBy;
        }
    }

    // The site's interface carries the command; the send names only the
    // console's own port.
    part def Site {
        part console : Console;
        part unit : Unit;

        interface link : CommandLink connect console.command to unit.command;
    }

    part site : Site;

    // A connector end reached through a multi-valued part denotes every element
    // it holds: each send delivers one message per unit, on that unit's own
    // identity.
    part def SquadSite {
        part console : Console;
        part units : Unit[2];

        connect console.command to units.command;
    }

    part squad : SquadSite;
}

// A callout is an occurrence: the individual that happened has a snapshot of
// the arrival and a timeslice of the approach.
package TeamMission {
    private import ScalarValues::*;
    private import Team::*;

    occurrence def Callout {
        attribute district : String;
        attribute crewOnSite : Integer;
        ref part unit : Robot;
    }

    individual def Callout17 :> Callout;

    individual callout17 : Callout17 {
        attribute :>> district = "Harbour";

        snapshot arrival {
            attribute :>> crewOnSite = 2;
        }

        timeslice approach {
            attribute :>> crewOnSite = 5;
        }
    }
}

// What the team is for, what is checked of it, and what is computed about it.
package TeamCases {
    private import ScalarValues::*;
    private import VerificationCases::VerdictKind;
    private import TradeStudies::*;
    private import SI::*;
    private import ISQ::*;
    private import Team::*;
    private import TeamComms::*;

    requirement def PayloadReq {
        subject truck : Truck;

        require constraint {
            truck.payload <= truck.payloadLimit
        }
    }

    // The mission the team exists to carry out, with the actors that carry it.
    use case def Disposal {
        subject site : Site;
        actor operator : Console;
        actor robot : Robot;

        objective {
            doc /* The device is disarmed and the team leaves with everyone. */
        }
    }

    use case disposal : Disposal {
        subject site = TeamComms::site;
    }

    part loadedTruck : Truck;

    // A calculation usage binds the inputs, so the question is asked of the
    // loaded truck rather than of arguments typed at the command line.
    calc usableReach : FleetReach {
        in robots = loadedTruck.robots;
        in floor = 2.0 [m];
    }

    calc teamEndurance : Endurance {
        in charges = loadedTruck.robots.charge;
        in draw = 40.0;
    }

    requirement payloadHolds : PayloadReq {
        subject truck = loadedTruck;
    }

    // A satisfaction assertion supplies the subject itself, so the requirement it
    // asserts leaves the subject unbound.
    requirement payloadAsserted : PayloadReq;

    assert satisfy payloadAsserted by loadedTruck;

    // A verification case verifies a requirement from its objective, and states
    // the verdict it reaches.
    verification def CheckPayload {
        subject truck : Truck;

        objective {
            verify payloadHolds;
        }

        return attribute verdict : VerdictKind;
    }

    verification checkPayload : CheckPayload {
        subject truck = loadedTruck;
    }

    // The heaviest robot pair the truck can still carry, for the solver to find.
    analysis def HeaviestPair {
        attribute robotMass : MassValue;
        attribute cradleMass : MassValue;

        objective heaviest : MaximizeObjective {
            subject :>> selectedAlternative;
            in calc :>> eval { robotMass }

            require constraint {
                2 * robotMass + 2 * cradleMass <= 1200 [kg]
            }

            require constraint {
                cradleMass >= 60 [kg]
            }
        }
    }
}
