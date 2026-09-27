// An analysis case whose action is performed by an external program: the
// `ToolExecution` metadata on `Solve` names the tool the `ThermalSolver`
// manifest entry registers, and `ToolVariable` on each parameter names it as
// the manifest's `variables` spell it. See README.md for the manifest and
// the commands that preview, run, record and report it.
package ThermalDemo {
    private import ScalarValues::*;
    private import ISQ::*;
    private import AnalysisTooling::*;

    part def Heater {
        attribute mass : MassValue;     // kg the heat spreads through
        attribute power : PowerValue;   // W the load draws
        attribute ambient : TemperatureValue; // K it starts and is held near
    }

    part box : Heater {
        attribute :>> mass = 12.5 [SI::kg];
        attribute :>> power = 250.0 [SI::W];
        attribute :>> ambient = 290.0 [SI::K];
    }

    // The requirement the objective is typed by: 400 K of headroom.
    requirement def ThermalMargin {
        subject h : Heater;
        in attribute tMax : TemperatureValue;
        attribute limit : TemperatureValue = 400.0 [SI::K];
        require constraint { tMax <= limit }
    }

    // The action the tool performs; its body never runs while the metadata is
    // present and the tool is registered.
    action def Solve {
        metadata ToolExecution {
            toolName = "ThermalSolver";
            uri = "thermal://demo/solve";
        }
        in mass : MassValue           { @ToolVariable { name = "mass"; } }
        in power : PowerValue         { @ToolVariable { name = "power"; } }
        in ambient : TemperatureValue { @ToolVariable { name = "ambient"; } }
        out tMax : TemperatureValue   { @ToolVariable { name = "tMax"; } }
        out margin : TemperatureValue { @ToolVariable { name = "margin"; } }
    }

    analysis def Heating {
        subject h : Heater;
        in attribute mass : MassValue = 12.5 [SI::kg];
        in attribute power : PowerValue = 250.0 [SI::W];
        in attribute ambient : TemperatureValue = 290.0 [SI::K];
        action solve : Solve {
            in mass = mass;
            in power = power;
            in ambient = ambient;
        }

        return tMax : TemperatureValue = solve.tMax;
        out margin : TemperatureValue = solve.margin;

        objective marginHeld : ThermalMargin {
            subject = h;
            in tMax = solve.tMax;
        }
    }

    analysis heating : Heating {
        subject h = box;
    }
}

// The package `-record-run` writes its records into, beside the case's own;
// the report's queries walk its direct members.
package Records {
    private import ScalarValues::*;
}

package Reporting {
    private import ScalarValues::*;
    private import DocumentQueries::*;

    // Every recorded heating run. `Project` reads the record's own features
    // only — an annotation's attribute values, `tools` among them, are not
    // projectable, so the tools a run reached are read in the record's source.
    calc def ThermalRuns :> Query {
        Project(
            source = WhereType(
            source = WhereMetadata(
            source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1),
            'metadata' = "AnalysisRecords::RecordedRun"),
            type = "AnalysisRecords::AnalysisRun"),
            properties = ("name", "caseName", "kind", "mass", "power", "ambient", "tMax", "margin", "objective"))
    }

    part def HeatingReport :> Document {
        attribute redefines title = "Heating runs an external tool answered";

        part intro : Paragraph {
            attribute redefines text = "Each row is a run of ThermalDemo::heating recorded with -record-run: the inputs bound, the outputs the ThermalSolver tool computed and the objective's verdict.";
        }

        part runs : Section {
            attribute redefines title = "Recorded runs";
            part table : Table {
                attribute redefines caption = "Every @AnalysisRecords::RecordedRun-annotated AnalysisRun in the Records package";
                calc rows : ThermalRuns;
            }
        }
    }
}
