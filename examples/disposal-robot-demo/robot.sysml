// A bomb-disposal robot in four packages: its structure, its behavior, a budget a
// solver can answer about, and its views. README.md walks through running it.

// The robot itself: its parts, the power and video between them, and which parts
// are safety-critical.
package Robot {
    private import ScalarValues::*;

    // Marks the parts a safety review has to look at, so a view can filter for them.
    metadata def Critical;

    enum def DriveMode {
        enum parked;
        enum creep;
        enum nominal;
    }

    enum def Link {
        enum tether;
        enum radio;
    }

    item def Charge;

    item def Frame;

    item def Device;

    port def PowerPort {
        in item drawn : Charge;
    }

    port def DataPort {
        in item frames : Frame;
    }

    connection def PowerLine {
        end source : PowerPort;
        end sink : PowerPort;
    }

    connection def DataLink {
        end producer : DataPort;
        end consumer : DataPort;
    }

    #Critical part def Battery {
        attribute capacity : Real = 1600.0;
        attribute charge : Real = 1200.0;

        out item supplied : Charge;
        port outlet : PowerPort;
    }

    part def PowerPack {
        attribute cells : Integer = 8;
        attribute output : Real = 340.0;

        out item generated : Charge;
        port outlet : PowerPort;
    }

    #Critical part def Wheel {
        attribute diameter : Real = 0.26;
        attribute steerable : Boolean = true;
    }

    #Critical part def Mobility {
        attribute drivePower : Real = 120.0;
        attribute speed : Real = 0.42;

        part wheels : Wheel[4];

        in item drawn : Charge;
        port feed : PowerPort;
    }

    part def Camera {
        attribute megapixels : Real = 20.2;

        out item captured : Frame;
        port imagery : DataPort;
    }

    part def Mast {
        attribute height : Real = 1.4;

        part camera : Camera;
    }

    part def Gripper {
        attribute grip : Real = 45.0;

        out item held : Device;
    }

    part def Manipulator {
        attribute reach : Real = 2.1;

        part gripper : Gripper;
    }

    #Critical part def Computer {
        attribute toolPower : Real = 60.0;

        attribute framesHeld : Integer = 0;

        in item frames : Frame;
        port imagery : DataPort;
    }

    part def Radio {
        attribute link : Link = Link::radio;
        attribute gain : Real = 26.0;
    }

    // The robot as a whole: the parts above, the power line and data link between
    // them, and the flows those connections carry.
    part def Platform {
        attribute mass : Real default 340.0;
        attribute mode : DriveMode = DriveMode::parked;

        part battery : Battery;
        part pack : PowerPack;
        part mobility : Mobility;
        part mast : Mast;
        part arm : Manipulator;
        part computer : Computer;
        part radio : Radio;

        connection drivePower : PowerLine connect battery.outlet to mobility.feed;
        connection imageryLink : DataLink connect mast.camera.imagery to computer.imagery;

        flow of Charge from battery.supplied to mobility.drawn;
        flow of Frame from mast.camera.captured to computer.frames;

        // What the response vehicle's ramp allows, checked per robot.
        assert constraint withinMassBudget {
            mass <= 400.0
        }
    }

    // Two robots, so a check has something to disagree about: the mock-up is over
    // its mass budget.
    part fielded : Platform {
        attribute :>> mass = 340.0;
    }

    part heavyMockup : Platform {
        attribute :>> mass = 460.0;
    }
}

// What the robot does: the calculations an approach is planned with, the action of
// one call-out, and the machine its modes are.
package RobotBehavior {
    private import ScalarValues::*;
    private import RealFunctions::sqrt;
    private import RealFunctions::min;
    private import SI::s;

    // Straight-line distance from the staging point to the device, in metres.
    calc def Approach {
        in dx : Real;
        in dy : Real;

        sqrt(dx * dx + dy * dy)
    }

    // Watt-hours a drive of that many metres costs at the given draw.
    calc energyForDrive {
        in metres : Real;
        in speed : Real;
        in drivePower : Real;

        drivePower * (metres / speed) / 3600.0
    }

    // How much of the call-out's video the operator link can carry back.
    calc streamable {
        in framesHeld : Integer;
        in linkMinutes : Real;

        min(framesHeld, linkMinutes * 4.0)
    }

    // One call-out: a fork runs the survey and the retrieval, the join waits for
    // both, and the decision at the end streams home what the budget allowed.
    action def DisposalRun {
        in attribute chargeAvailable : Real = 1200.0;

        attribute surveyFrames : Integer = 0;
        attribute armFrames : Integer = 0;
        attribute surveyCost : Real = 0.0;
        attribute handlingCost : Real = 0.0;

        out attribute framesHeld : Integer;
        out attribute streamed : Integer;
        out attribute chargeLeft : Real;

        first start;

        action powerOn {
            assign streamed := 0;
        }

        fork split;

        // Each branch writes features of its own, so the join sees the same values
        // whatever order they interleaved in.
        action survey {
            assign surveyFrames := 12;
            assign surveyCost := 40.0;
        }

        // A node with a flow of its own: the gripper closes, then the device is
        // stowed, and only then does the branch reach the join.
        action retrieve {
            action grip {
                assign handlingCost := 120.0;
            }

            action stow {
                assign armFrames := 3;
            }

            first grip;
            succession first grip then stow;
        }

        join sync;

        action tally {
            assign framesHeld := surveyFrames + armFrames;
            assign chargeLeft := chargeAvailable - surveyCost - handlingCost;
        }

        action stream {
            assign streamed := framesHeld;
        }

        action holdVideo {
            assign streamed := 0;
        }

        succession first start then powerOn;
        succession first powerOn then split;
        succession first split then survey;
        succession first split then retrieve;
        succession first survey then sync;
        succession first retrieve then sync;
        succession first sync then tally;

        // Stream home only what the call-out's power budget left room for.
        succession first tally then budget;
        decide budget;
        if chargeLeft > 900.0 then stream;
        else holdVideo;

        succession first stream then done;
        succession first holdVideo then done;
    }

    // The robot's modes: `%advance` dispatches the timed triggers, and each entry
    // action writes a feature of the machine.
    state def Modes {
        attribute odometer : Real = 0.0;
        attribute framesCaptured : Integer = 0;
        attribute faults : Integer = 0;

        entry; then idle;

        state idle;

        // A state with substates of its own: entering `rolling` enters `approach`
        // around it, and a fault takes the guarded transition out.
        state approach {
            state rolling {
                entry action roll {
                    assign odometer := odometer + 21.0;
                }
            }

            state holding;
        }

        state handling {
            entry action inspect {
                assign framesCaptured := framesCaptured + 12;
            }
        }

        state safing {
            entry action record {
                assign faults := faults + 1;
            }
        }

        // A transition into a substate names it through the state around it.
        transition idle_to_rolling
            first idle
            accept after 5 [s]
            then approach.rolling;

        transition rolling_to_holding
            first approach.rolling
            accept after 10 [s]
            then approach.holding;

        transition rolling_to_safe
            first approach.rolling
            if faults > 0
            then safing;

        transition approach_to_handling
            first approach
            accept after 20 [s]
            then handling;

        transition first handling then done;
    }

    // What each video stream added, kept beside the controller rather than in it.
    part def StreamLog {
        attribute frames : Integer = 0;
        attribute passes : Integer = 0;
    }

    // An object of this type runs the machine: `%instantiate` starts it, and
    // `%invoke` calls the operation below on that object.
    part def RobotController {
        attribute odometer : Real = 0.0;
        attribute framesHeld : Integer = 0;

        part log : StreamLog;

        exhibit state modes : Modes;

        action recordPass {
            in attribute frames : Integer;

            // The tally is the controller's own; the log is a part of it, reached
            // by the chain the assignment names.
            action store {
                assign framesHeld := framesHeld + frames;
                assign log.frames := log.frames + frames;
                assign log.passes := log.passes + 1;
            }

            first store;
        }
    }

    part controller : RobotController;
}

// The call-out budget as conditions a solver can answer about: how the energy is
// split, which plans cannot hold, which builds are consistent, which plan is best.
package RobotSolver {
    private import ScalarValues::*;
    public import SI::*;
    public import ISQ::*;
    private import TradeStudies::*;

    doc /* Energy is in watt-hours, mass in kilograms, distance in metres. */

    // A plan for one call-out, with the three draws it splits the battery into.
    part def RunPlan {
        attribute energyAvailable : Integer = 1200;
        attribute driveEnergy : Integer;
        attribute toolEnergy : Integer;
        attribute housekeepingEnergy : Integer;

        assert constraint fitsTheBattery {
            driveEnergy + toolEnergy + housekeepingEnergy <= energyAvailable
        }

        assert constraint keepsTheRobotAlive {
            housekeepingEnergy >= 300
        }

        assert constraint driveIsWorthIt {
            driveEnergy >= 200
        }

        assert constraint toolWorkIsWorthIt {
            toolEnergy >= 150
        }
    }

    // One requirement states the whole budget: `%check` answers sat and `%solve`
    // gives a witness plan.
    requirement def RunBudget {
        subject plan : RunPlan;

        require constraint {
            plan.driveEnergy + plan.toolEnergy + plan.housekeepingEnergy <= 1200
        }

        require constraint {
            plan.housekeepingEnergy >= 300
        }

        require constraint {
            plan.driveEnergy >= 200
        }

        require constraint {
            plan.toolEnergy >= 150
        }
    }

    // A call-out nobody can plan: `%check` answers unsat and `%explain` names the
    // conditions that conflict.
    requirement def OverbookedRun {
        subject plan : RunPlan;

        require constraint {
            plan.driveEnergy + plan.toolEnergy + plan.housekeepingEnergy <= 1200
        }

        require constraint {
            plan.driveEnergy >= 600
        }

        require constraint {
            plan.toolEnergy >= 500
        }

        require constraint {
            plan.housekeepingEnergy >= 300
        }
    }

    requirement runIsBudgeted : RunBudget;

    // Two plans the budget is asserted of, one of which fails a condition.
    part run12 : RunPlan {
        attribute :>> driveEnergy = 420;
        attribute :>> toolEnergy = 380;
        attribute :>> housekeepingEnergy = 340;
    }

    part run13 : RunPlan {
        attribute :>> driveEnergy = 700;
        attribute :>> toolEnergy = 380;
        attribute :>> housekeepingEnergy = 340;
    }

    part plans {
        assert satisfy runIsBudgeted by run12;
        assert satisfy runIsBudgeted by run13;
    }

    attribute def Wheels;

    attribute def Uplink;

    attribute def Tool;

    part def RobotBuild {
        attribute wheels : Wheels;
        attribute uplink : Uplink;
        attribute tool : Tool;
    }

    // Three variation points whose choices interact, which `%configure` selects
    // among and enumerates.
    part robotFamily : RobotBuild {
        variation attribute :>> wheels {
            variant attribute aluminium;
            variant attribute titanium;
        }

        variation attribute :>> uplink {
            variant attribute tetherOnly;
            variant attribute radio;
        }

        variation attribute :>> tool {
            variant attribute camera;
            variant attribute gripper;
        }

        // A gripper needs the operator watching from cover, and the heavier wheels
        // to carry it over rubble.
        assert constraint buildIsConsistent {
            (tool == tool::gripper implies uplink == uplink::radio)
                and (tool == tool::gripper implies wheels == wheels::titanium)
        }
    }

    // The most tool time a call-out can hold, within the conditions stated.
    analysis def BestRun {
        attribute driveEnergy : Integer;
        attribute toolEnergy : Integer;
        attribute housekeepingEnergy : Integer;

        objective mostToolTime : MaximizeObjective {
            subject :>> selectedAlternative;
            require constraint {
                driveEnergy + toolEnergy + housekeepingEnergy <= 1200
            }

            require constraint {
                housekeepingEnergy >= 300
            }

            // The robot has to reach the device before it can work on it.
            require constraint {
                driveEnergy >= 200
            }

            in calc :>> eval { toolEnergy }
        }
    }

    // A quantity-valued objective, whose optimum is reported as a quantity.
    analysis def LightestRobot {
        attribute mass : ISQ::MassValue;

        objective lightest : MinimizeObjective {
            subject :>> selectedAlternative;
            require constraint {
                mass >= 300 [kg]
            }

            require constraint {
                mass <= 420 [kg]
            }

            in calc :>> eval { mass }
        }
    }
}

// How the model is presented: one view per rendering kind, a viewpoint the robots
// are checked against, and a view reaching only the critical parts.
package RobotViews {
    private import Views::*;
    private import StandardViewDefinitions::*;

    // What a reviewer wants of any robot exposed: the team can carry it to the scene.
    concern def MassBudget {
        subject robot : Robot::Platform;
        attribute maxMass = 400.0;
        require constraint {
            robot.mass <= maxMass
        }
    }

    viewpoint def OperationsPerspective {
        frame concern mass : MassBudget;
    }

    viewpoint operationsPerspective : OperationsPerspective;

    // The parts of the platform and the connections between them.
    view interfaces {
        expose Robot::Platform;
        render asInterconnectionDiagram;
    }

    // The rendering kind comes from the standard view definition each specializes.
    view modes : StateTransitionView {
        expose RobotBehavior::Modes;
    }

    view run : ActionFlowView {
        expose RobotBehavior::DisposalRun;
    }

    view partsTable {
        expose Robot::Platform;
        render asElementTable;
    }

    // A filtered recursive expose, reaching only the parts marked critical.
    view criticalParts {
        expose Robot::**[@Robot::Critical];
    }

    // States no rendering, so a tree renders it, with the interconnection view
    // nested. `%view` checks the robots exposed against the concern framed here.
    view overview {
        expose Robot::fielded;
        expose Robot::heavyMockup;
        view interfaceSubview :> interfaces;

        satisfy operationsPerspective;
        frame concern mass : MassBudget;
    }
}
