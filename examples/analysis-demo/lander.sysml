// A lander's powered descent, asked four ways: an analysis case, a verification
// case, a trade study over three candidates, and two behaviors on one clock.
// Values are plain Reals in SI base units so the tables read without suffixes.

package Landers {
    private import ScalarValues::*;

    part def Lander {
        attribute dryMass : Real;          // kg, without fuel
        attribute fuel : Real;             // kg loaded at the start of descent
        attribute burnRate : Real;         // kg/s the engine consumes at full thrust
        attribute payload : Real;          // kg of instruments carried
        attribute touchdownSpeed : Real;   // m/s, from the descent profile

        attribute braking : Boolean = false;

        // Coast, then brake once the timer expires; an instantiated part runs it.
        exhibit state flightMode {
            entry; then coasting;
            state coasting;
            transition first coasting accept after 5 [SI::s] then decelerating;
            state decelerating {
                entry assign braking := true;
            }
        }
    }

    part scout : Lander {
        attribute :>> dryMass = 600.0;
        attribute :>> fuel = 250.0;
        attribute :>> burnRate = 3.0;
        attribute :>> payload = 40.0;
        attribute :>> touchdownSpeed = 1.2;
    }
    part hauler : Lander {
        attribute :>> dryMass = 1400.0;
        attribute :>> fuel = 900.0;
        attribute :>> burnRate = 8.0;
        attribute :>> payload = 300.0;
        attribute :>> touchdownSpeed = 1.9;
    }
    part relay : Lander {
        attribute :>> dryMass = 450.0;
        attribute :>> fuel = 180.0;
        attribute :>> burnRate = 2.5;
        attribute :>> payload = 25.0;
        attribute :>> touchdownSpeed = 1.4;
    }
}

package Descent {
    private import ScalarValues::*;
    private import Landers::*;

    // The requirement the analysis objective is typed by; the case binds its subject.
    requirement def FuelReserve {
        subject lander : Lander;
        in attribute fuelLeft : Real;
        attribute reserve : Real = 20.0;
        require constraint { fuelLeft >= reserve }
    }

    // Fuel a descent burn leaves: `then`-chained action steps feed each other
    // through parameter bindings, and the case's outputs are read afterwards.
    analysis def FuelBudget {
        subject lander : Lander;
        in attribute burnTime : Real = 40.0;   // s of engine burn

        action burn {
            in rate : Real = lander.burnRate;
            in seconds : Real = burnTime;
            out used : Real;
            assign used := rate * seconds;
        }
        then action remaining {
            in loaded : Real = lander.fuel;
            in used : Real = burn.used;
            out left : Real;
            assign left := loaded - used;
        }

        out fuelUsed : Real = burn.used;
        out wetMass : Real = lander.dryMass + remaining.left;

        objective reserveHeld : FuelReserve {
            subject = lander;
            in fuelLeft = remaining.left;
        }

        return fuelLeft : Real = remaining.left;
    }

    // A usage that binds its subject runs without an object on the command line.
    analysis scoutBudget : FuelBudget {
        subject lander = scout;
    }

    // The touchdown requirement, stated of the scout.
    requirement def SoftLanding {
        subject lander : Lander;
        attribute limit : Real = 1.5;
        require constraint { lander.touchdownSpeed <= limit }
    }
    requirement scoutLandsSoftly : SoftLanding {
        subject lander = scout;
    }

    // Verifies the requirement from its objective; the body's `PassIf` decides the verdict.
    verification def TouchdownCheck {
        subject lander : Lander;
        objective {
            verify scoutLandsSoftly;
            require constraint { lander.touchdownSpeed <= 1.5 }
        }
        VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
    }
    verification checkScout : TouchdownCheck {
        subject lander = scout;
    }
}

package Selection {
    private import ScalarValues::*;
    private import TradeStudies::*;
    private import Landers::*;

    // Lightest on the pad: every alternative is scored, and the objective picks the least.
    analysis lightest : TradeStudy {
        subject : Lander[1..*] = (scout, hauler, relay);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part l :>> alternative : Lander;
            return :>> result : Real = l.dryMass + l.fuel;
        }
        return part :>> selectedAlternative : Lander;
    }

    // Most payload once fuel is paid for; `fuelCost` is a case parameter, so it sweeps.
    analysis mostValuable : TradeStudy {
        subject : Lander[1..*] = (scout, hauler, relay);
        in attribute fuelCost : Real;   // kg of payload one kg of fuel is worth
        objective : MaximizeObjective;
        calc :>> evaluationFunction {
            in part l :>> alternative : Lander;
            return :>> result : Real = l.payload - fuelCost * l.fuel;
        }
        return part :>> selectedAlternative : Lander;
    }
}

package Timing {
    private import ScalarValues::*;
    private import Landers::*;

    // Waits five seconds on the clock the scout's flight mode also runs on, then
    // reads whether it is braking; both are due at t=5.0, so the order is a choice.
    action def GroundWatch {
        out armed : Boolean = false;
        out sawBraking : Boolean = false;

        first start;
        then action arm assign armed := scout.braking == false;
        then action wait accept after 5 [SI::s];
        then action look assign sawBraking := scout.braking;
        then done;
    }
    action groundWatch : GroundWatch;

    // The same watch as an analysis step; the bound subject is the scout it reads.
    analysis watchDescent {
        subject lander = scout;
        action watch : GroundWatch;
        out sawBraking : Boolean = watch.sawBraking;
    }
}
