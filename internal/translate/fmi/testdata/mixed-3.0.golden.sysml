// Imported from mixed-3.0.fmu (FMI 3.0, co-simulation, model identifier mixer)
package Mixer {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;
	private import SI::*;

	calc def Mixer {
		doc /* A mixed I/O model. */
		metadata ToolExecution {
			toolName = "fmi";
			uri = "file:///opt/fmus/mixed-3.0.fmu";
		}
		// parameters and inputs of the model, in document order
		in m : MassValue = 2.5 [kg] { @ToolVariable { name = "m"; } } // Mass
		in mode : Integer = 1 { @ToolVariable { name = "mode"; } }
		// tick Clock not imported: Clock is not an imported type
		// array vec with a structural dimension not imported
		// structural parameter n not imported
		// the simulation experiment
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 1.0 { @ToolVariable { name = "fmi:stopTime"; } }
		// outputs at stopTime
		return count : Integer { @ToolVariable { name = "count"; } } // Count
		out ok : Boolean { @ToolVariable { name = "ok"; } }
		out label : String { @ToolVariable { name = "label"; } }
		// blob Binary not imported: Binary is not an imported type
		out grid : Real[4] nonunique { @ToolVariable { name = "grid"; } } // kg
		// multi-dimensional array matrix not imported
	}
}
