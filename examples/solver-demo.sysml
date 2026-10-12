// A rover's power, mass and configuration budget, written so that each solver
// command has something to answer: see SOLVER-DEMO.md for the walkthrough.
package SolverDemo {
    private import ScalarValues::*;
    public import SI::*;
    public import ISQ::*;
    private import TradeStudies::*;

    doc /* Power is in watts, mass in kilograms. */

    enum def Radio {
        enum lowGain;
        enum highGain;
    }

    // Conditions a solver can satisfy: `%check` answers sat and `%solve` fills in
    // the powers, keeping the capacity the model already states.
    part def Rover {
        attribute batteryCapacity : Integer = 1500;
        attribute drivePower : Integer;
        attribute sciencePower : Integer;
        attribute radio : Radio;

        assert constraint powerFitsBudget {
            drivePower + sciencePower <= 220
        }

        assert constraint driveIsUseful {
            drivePower >= 60
        }

        assert constraint scienceIsUseful {
            sciencePower >= 40
        }

        assert constraint highGainCostsPower {
            radio == Radio::highGain implies sciencePower >= 120
        }
    }

    // A requirement states several conditions at once, so one query is about all
    // of them: `%check` and `%solve` answer for the budget as a whole.
    requirement def PowerBudgetRequirement {
        subject rover : Rover;

        require constraint {
            rover.drivePower + rover.sciencePower <= 220
        }

        require constraint {
            rover.drivePower >= 60
        }

        require constraint {
            rover.sciencePower >= 40
        }
    }

    // Conditions that cannot hold together: `%check` answers unsat and `%explain`
    // names the three of them that conflict.
    requirement def OverbookedBudget {
        subject rover : Rover;

        require constraint {
            rover.drivePower + rover.sciencePower <= 200
        }

        require constraint {
            rover.drivePower >= 150
        }

        require constraint {
            rover.sciencePower >= 90
        }
    }

    // A satisfaction assertion is a query too: `%check` asks whether the object
    // can satisfy the requirement asserted of it.
    part rover1 : Rover {
        attribute :>> drivePower = 90;
        attribute :>> sciencePower = 100;
        attribute :>> radio = Radio::lowGain;

        assert satisfy requirement powerIsBudgeted : PowerBudgetRequirement;
    }

    attribute def Chassis;

    attribute def Antenna;

    part def RoverPlatform {
        attribute chassis : Chassis;
        attribute antenna : Antenna;
    }

    // Two variation points whose choices interact, so `%configure … all` reports
    // three consistent selections of the four combinations.
    part roverFamily : RoverPlatform {
        variation attribute :>> chassis {
            variant attribute light;
            variant attribute rugged;
        }

        variation attribute :>> antenna {
            variant attribute fixed;
            variant attribute steerable;
        }

        assert constraint steerableNeedsRugged {
            antenna == antenna::steerable implies chassis == chassis::rugged
        }
    }

    // An objective `%optimize` improves within the conditions the case requires.
    analysis def PowerBudget {
        attribute drivePower : Integer;
        attribute sciencePower : Integer;

        assert constraint {
            drivePower + sciencePower <= 220
        }

        assert constraint {
            drivePower >= 60
        }

        assert constraint {
            sciencePower >= 40
        }

        objective mostScience : MaximizeObjective {
            subject :>> selectedAlternative;
            in calc :>> eval { sciencePower }
        }
    }

    // A quantity-valued objective, whose optimum is reported in the unit the
    // conditions are written in.
    analysis def MassBudget {
        attribute mass : ISQ::MassValue;

        assert constraint {
            mass >= 780 [kg]
        }

        assert constraint {
            mass <= 950 [kg]
        }

        objective lightest : MinimizeObjective {
            subject :>> selectedAlternative;
            in calc :>> eval { mass }
        }
    }
}
