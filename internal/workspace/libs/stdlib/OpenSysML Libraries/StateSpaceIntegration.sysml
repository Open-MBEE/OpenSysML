library package StateSpaceIntegration {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG SysML
	 * standard library: it names what the runtime provides to run a model written against
	 * StateSpaceRepresentation by fixed time steps. The vendored OMG files are unmodified.
	 *
	 * A model specializing ContinuousStateSpaceDynamics or DiscreteStateSpaceDynamics runs
	 * when it also specializes FixedStepDynamics, which states the step it advances by and
	 * the time it stops at. Each step advances the clock the run's actions and state
	 * machines share, so their timed accepts and the steps interleave in order of time.
	 */

	private import StateSpaceRepresentation::*;
	private import ISQ::DurationValue;
	private import ScalarValues::*;

	calc def Euler :> Integrate {
		doc
		/*
		 * The forward Euler method: the state advances by timeStep times the derivative
		 * at the start of the step. First-order accurate; its error shrinks with the step.
		 */
	}

	calc def RK4 :> Integrate {
		doc
		/*
		 * The classical fourth-order Runge-Kutta method: the derivative is evaluated at the
		 * start, twice at the midpoint and at the end of the step, and the state advances by
		 * their weighted mean. The default integrator where a model names none.
		 */
	}

	abstract action def FixedStepDynamics :> StateSpaceDynamics {
		doc
		/*
		 * A dynamics run by fixed time steps: from the clock's time when it starts, each
		 * step advances the state and the clock by timeStep, until the clock reaches
		 * stopTime. Without a stopTime the run continues while the clock is advanced.
		 * time is the clock's instant at the latest step, for a guard to read; while an
		 * integrator evaluates getDerivative it is the instant of that stage of the step.
		 */
		attribute timeStep : DurationValue;
		attribute stopTime : DurationValue[0..1];
		attribute time : DurationValue;
	}

	action def ZeroCrossing :> ZeroCrossingEventDef {
		doc
		/*
		 * A zero crossing of guard, a real-valued expression over the state, the input
		 * and the time, watched between consecutive steps: the event occurs when the guard's
		 * sign after a step differs from its sign before it, or the guard is exactly
		 * zero after a step. The crossing is located at the end of the step that
		 * detected it. A terminal crossing ends the dynamics at that step.
		 */
		attribute guard : Real;
		attribute terminal : Boolean default false;
	}
}
