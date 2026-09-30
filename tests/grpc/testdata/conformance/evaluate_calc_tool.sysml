package thermoCase {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;

	calc def Thermal {
		metadata ToolExecution { toolName = "Thermo"; uri = "thermo://local"; }
		in m : Real[1] { @ToolVariable { name = "mass"; } }
		in p : Real[1] { @ToolVariable { name = "power"; } }
		out warn : Boolean[1] { @ToolVariable { name = "warn"; } }
		return : TemperatureValue[1] { @ToolVariable { name = "Tmax"; } }
	}
}
