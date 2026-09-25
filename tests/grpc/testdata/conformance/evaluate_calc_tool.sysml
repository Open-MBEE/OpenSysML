package thermoCase {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;

	calc def Thermal {
		metadata ToolExecution { toolName = "Thermo"; uri = "thermo://local"; }
		in m : Real { @ToolVariable { name = "mass"; } }
		in p : Real { @ToolVariable { name = "power"; } }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		return : TemperatureValue { @ToolVariable { name = "Tmax"; } }
	}
}
