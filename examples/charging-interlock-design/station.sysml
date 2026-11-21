package ChargingInterlock {
    private import ScalarValues::*;
    private import ModelingMetadata::*;
    private import SI::*;

    attribute def VehicleConnected;
    attribute def LockDetected;
    attribute def ChargeRequested;
    attribute def LockLost;

    constraint def LockGatesEnergy {
        in lockSeen : Boolean;
        in outputEnergized : Boolean;
        not outputEnergized or lockSeen
    }

    part def StationController {
        port vehicle;
        port lock;
        port powerStage;
        attribute linkLocked : Boolean = false;
        attribute energized : Boolean = false;
        constraint powerRequiresLock : LockGatesEnergy {
            in lockSeen = linkLocked;
            in outputEnergized = energized;
        }

        exhibit state Interlock {
            entry; then disconnected;
            state disconnected;
            state awaitingLock;
            state armed { entry action { assign linkLocked := true; } }
            state supplying { entry action { assign energized := true; } }
            state inhibited {
                entry action {
                    assign energized := false;
                    assign linkLocked := false;
                }
            }

            transition first disconnected accept VehicleConnected then awaitingLock;
            transition first awaitingLock accept ChargeRequested then awaitingLock;
            transition first awaitingLock accept LockDetected then armed;
            transition first armed accept ChargeRequested then supplying;
            transition first armed accept LockLost then inhibited;
            transition first supplying accept LockLost then inhibited;
        }
    }

    requirement <'INT-01'> powerGate {
        doc /* The station controller shall only energize its output while it recognizes the connector lock. */
        subject station : StationController;
        require constraint condition : LockGatesEnergy {
            in lockSeen = station.linkLocked;
            in outputEnergized = station.energized;
        }
    }
    #refinement dependency interlockBehavior from StationController::Interlock to powerGate;

    part rig {
        part vehicleInterface { port station; }
        part lockSensor { port output; }
        part powerStage { port control; }
        part controller : StationController;

        connect vehicleInterface.station to controller.vehicle;
        connect lockSensor.output to controller.lock;
        connect controller.powerStage to powerStage.control;
    }

    // Bounded design environment: the vehicle connects first. Its first request
    // and the sensor's lock report race at one instant, then the vehicle retries.
    action designEnvironment {
        first start;
        then action connectVehicle send new VehicleConnected() to rig.controller;
        then action waitForConnection accept after 1 [s];
        then fork arrivalRace;
        action request send new ChargeRequested() to rig.controller;
        action acquire send new LockDetected() to rig.controller;
        join reported;
        succession arrivalRace then request;
        succession arrivalRace then acquire;
        succession request then reported;
        succession acquire then reported;
        succession reported then waitForRetry;
        action waitForRetry accept after 1 [s];
        succession waitForRetry then retry;
        action retry send new ChargeRequested() to rig.controller;
        succession retry then done;
    }
}
