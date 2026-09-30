// A multi-valued tool output recorded by -record-run and tabulated by a
// document query: the CSV reply's every data record is one item of the
// sequence `temps` binds.
package Profiles {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	action def ThermalProfile {
		metadata ToolExecution {
			toolName = "Thermal";
			uri = "thermal://host/profile";
		}
		out temps : TemperatureValue[0..*] nonunique { @ToolVariable { name = "temps"; } }
	}

	analysis def ProfileCase {
		action step : ThermalProfile;
		out temps : TemperatureValue[0..*] nonunique = step.temps;
	}
	analysis Case : ProfileCase;
}

package Records {
}

package Reporting {
	private import ScalarValues::*;
	private import DocumentQueries::*;

	calc def AllRuns :> Query {
		Project(
			source = WhereMetadata(
				source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1),
				'metadata' = "AnalysisRecords::RecordedRun"),
			properties = ("name", "temps"))
	}

	part def Doc :> Document {
		attribute redefines title = "Recorded temperature profiles";
		part runs : Table {
			attribute redefines caption = "The recorded run's multi-valued output";
			calc rows : AllRuns;
		}
	}
}
