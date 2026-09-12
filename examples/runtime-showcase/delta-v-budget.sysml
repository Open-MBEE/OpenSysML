// A delta-v budget computed from the stages of a rocket, units carried through
// every step, and an analysis deciding whether it reaches orbit with margin.
package DeltaVBudget {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;
    private import OpenSysMLMathFunctions::ln;

    attribute standardGravity :> ISQ::acceleration = 9.80665 ['m/s²'];

    // Tsiolkovsky: Δv = Isp · g0 · ln(m0 / mf). Seconds times m/s² is a speed;
    // the mass ratio is dimensionless, as ln requires.
    calc def RocketEquation {
        in isp :> ISQ::time;
        in g0 :> ISQ::acceleration;
        in m0 :> ISQ::mass;
        in mf :> ISQ::mass;
        return deltaV :> ISQ::speed = isp * g0 * ln(m0 / mf);
    }

    part def Stage {
        attribute dryMass :> ISQ::mass;
        attribute propellantMass :> ISQ::mass;
        attribute specificImpulse :> ISQ::time;
    }

    // The delta-v a stage delivers while lifting the stack above it.
    calc def StageDeltaV {
        in stage : Stage;
        in payload :> ISQ::mass;
        return :> ISQ::speed = RocketEquation(
            stage.specificImpulse,
            standardGravity,
            stage.dryMass + stage.propellantMass + payload,
            stage.dryMass + payload
        );
    }

    part def TwoStageRocket {
        part stage1 : Stage;
        part stage2 : Stage;
        attribute payload :> ISQ::mass;
        attribute stage1DeltaV :> ISQ::speed =
            StageDeltaV(stage1, stage2.dryMass + stage2.propellantMass + payload);
        attribute stage2DeltaV :> ISQ::speed = StageDeltaV(stage2, payload);
        attribute totalDeltaV :> ISQ::speed = stage1DeltaV + stage2DeltaV;
    }

    part saturnIB : TwoStageRocket {
        part :>> stage1 : Stage {
            attribute :>> dryMass = 45000 [kg];
            attribute :>> propellantMass = 400000 [kg];
            attribute :>> specificImpulse = 263 [s];
        }
        part :>> stage2 : Stage {
            attribute :>> dryMass = 10000 [kg];
            attribute :>> propellantMass = 105000 [kg];
            attribute :>> specificImpulse = 421 [s];
        }
        attribute :>> payload = 18000 [kg];
    }

    requirement def OrbitWithMargin {
        doc /* The vehicle reaches low Earth orbit with delta-v to spare. */
        subject rocket : TwoStageRocket;
        attribute required :> ISQ::speed default = 9400 ['m/s'];
        attribute margin :> ISQ::speed = rocket.totalDeltaV - required;
        require constraint { margin > 0 ['m/s'] }
    }

    analysis def AscentBudget {
        subject rocket : TwoStageRocket;
        in required :> ISQ::speed = 9400 ['m/s'];
        out stage1DeltaV :> ISQ::speed = rocket.stage1DeltaV;
        out stage2DeltaV :> ISQ::speed = rocket.stage2DeltaV;
        out margin :> ISQ::speed = rocket.totalDeltaV - required;
        objective reachesOrbit : OrbitWithMargin {
            subject :>> rocket = AscentBudget::rocket;
            attribute :>> required = AscentBudget::required;
        }
    }

    analysis saturnIBAscent : AscentBudget { subject rocket = saturnIB; }

    requirement saturnIBOrbit : OrbitWithMargin;
    assert satisfy saturnIBOrbit by saturnIB;

    // The same physics with one argument dropped. An invocation may leave a
    // parameter unbound, so this is well-formed; it can never be evaluated.
    calc def StageDeltaVWithoutGravity {
        in stage : Stage;
        in payload :> ISQ::mass;
        return :> ISQ::speed = RocketEquation(
            stage.specificImpulse,
            stage.dryMass + stage.propellantMass + payload,
            stage.dryMass + payload
        );
    }

    // μ = GM has dimension L³·T⁻², not force. Typed as a force, the square
    // root comes out as M½·T⁻¹, which is not a speed.
    calc def InjectionDeltaV {
        in mu :> ISQ::force;
        in r_leo :> ISQ::length;
        in r_apoapsis :> ISQ::length;
        return :> ISQ::speed =
            (mu * (2 / r_leo - 2 / (r_leo + r_apoapsis)))^(1/2) - (mu / r_leo)^(1/2);
    }
}
