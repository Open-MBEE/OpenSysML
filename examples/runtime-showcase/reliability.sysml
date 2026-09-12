// Mission reliability R = e^(-λt) rolled up over critical components, with a
// requirement on the result. Two spellings of e^x; both resolve, one evaluates.
package Reliability {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;
    private import OpenSysMLMathFunctions::exp;
    private import RealFunctions::product;
    private import ControlFunctions::collect;

    abstract part def Component {
        attribute failureRate :> ISQ::frequency;
    }

    part def GuidanceComputer :> Component { attribute :>> failureRate = 8.0E-9 [Hz]; }
    part def FuelCell :> Component { attribute :>> failureRate = 4.0E-9 [Hz]; }
    part def RadioTransponder :> Component { attribute :>> failureRate = 2.0E-9 [Hz]; }

    // λ in Hz times t in s is a pure number, which is what exp takes.
    calc def SurvivalProbability {
        in failureRate :> ISQ::frequency;
        in missionTime :> ISQ::time;
        return : Real = exp(-(failureRate * missionTime));
    }

    part def Spacecraft {
        part guidance : GuidanceComputer;
        part power : FuelCell;
        part comms : RadioTransponder;
        part critical : Component[*] = (guidance, power, comms);
        attribute missionTime :> ISQ::time;
        attribute reliability : Real = product(critical.failureRate->collect {
            in rate :> ISQ::frequency;
            SurvivalProbability(rate, missionTime)
        });
    }

    part apollo : Spacecraft {
        attribute :>> missionTime = 195 [h];
    }

    requirement def CrewSafetyReliability {
        doc /* Every critical component survives the mission with high probability. */
        subject craft : Spacecraft;
        attribute threshold : Real default = 0.99;
        require constraint { craft.reliability >= threshold }
    }

    // Eighty-four days aloft with the same hardware.
    part longStay : Spacecraft {
        attribute :>> missionTime = 84 [d];
    }

    requirement crewSafety : CrewSafetyReliability;
    assert satisfy crewSafety by apollo;
    assert satisfy crewSafety by longStay;

    // The same formula as a power of `eulerNumber`, which ISQ::* resolves to
    // the ISO 80000-11 flow characteristic, a quantity with no value, not e.
    calc def SurvivalProbabilityFromConstant {
        in failureRate :> ISQ::frequency;
        in missionTime :> ISQ::time;
        return : Real = eulerNumber^(-(failureRate * missionTime));
    }
}
