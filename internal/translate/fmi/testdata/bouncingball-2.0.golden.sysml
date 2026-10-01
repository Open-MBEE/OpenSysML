// Imported from bouncingball-2.0.fmu (FMI 2.0, co-simulation, model identifier BouncingBall)
package BouncingBall {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;
	private import SI::*;

	calc def BouncingBall {
		doc /* A bouncing ball. */
		metadata ToolExecution {
			toolName = "fmi";
			uri = "file:///opt/fmus/bouncingball-2.0.fmu";
		}
		// parameters and inputs of the model, in document order
		in g : AccelerationValue[1] = -9.81 [m/s^2] { @ToolVariable { name = "g"; } }
		in e : Real[1] = 0.7 { @ToolVariable { name = "e"; } }
		// the simulation experiment
		in startTime : Real[1] = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real[1] = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		in stepSize : Real[1] = 0.01 { @ToolVariable { name = "fmi:stepSize"; } }
		in tolerance : Real[1] = 0.0001 { @ToolVariable { name = "fmi:tolerance"; } }
		// outputs at stopTime
		return h : LengthValue[1] { @ToolVariable { name = "h"; } } // Height of the ball
		out v : SpeedValue[1] { @ToolVariable { name = "v"; } } // Velocity
	}
}
