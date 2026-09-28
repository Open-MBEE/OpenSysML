// Imported from bouncingball-2.0.fmu (FMI 2.0, co-simulation, model identifier BouncingBall)
package BouncingBall {
	private import ScalarValues::*;
	private import AnalysisTooling::*;

	calc def BouncingBall {
		doc /* A bouncing ball. */
		metadata ToolExecution {
			toolName = "fmi";
			uri = "file:///opt/fmus/bouncingball-2.0.fmu";
		}
		// parameters and inputs of the model, in document order
		in g : Real = -9.81 { @ToolVariable { name = "g"; } } // m/s2
		in e : Real = 0.7 { @ToolVariable { name = "e"; } }
		// the simulation experiment
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		in stepSize : Real = 0.01 { @ToolVariable { name = "fmi:stepSize"; } }
		in tolerance : Real = 0.0001 { @ToolVariable { name = "fmi:tolerance"; } }
		// outputs at stopTime
		return h : Real { @ToolVariable { name = "h"; } } // m — Height of the ball
		out v : Real { @ToolVariable { name = "v"; } } // m/s — Velocity
	}
}
