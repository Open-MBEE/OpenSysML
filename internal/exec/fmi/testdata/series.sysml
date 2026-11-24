// A tool:fmi run sampled into a trajectory, with two peak-temperature checks.
// The uri placeholder is filled with the FMU file URI by the Go fixture.
package Drive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def Traj {
		metadata ToolExecution { toolName = "fmi"; uri = "%s"; }
		in samples : Integer = 5 { @ToolVariable { name = "fmi:samples"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return temps : Real[1..*] nonunique { @ToolVariable { name = "fmi:series:T"; } }
	}

	requirement def Peak {
		in temps : Real[1..*] nonunique;
		in limit : Real;
		require constraint { temps->maximize { in t; t } <= limit }
	}

	requirement def PeakFromRun {
		in limit : Real;
		require constraint { traj()->maximize { in t; t } <= limit }
	}

	calc traj : Traj { }
}
