// A send may construct a bundled library type: `new ToolExecution(toolName = "x")`
// binds the library's own attribute, and the accepting side reads it back from
// the materialized object.
package test {
	private import ScalarValues::*;
	private import AnalysisTooling::*;

	port def Link;

	part def Probe {
		port link : Link;

		exhibit state beacon {
			entry; then sending;

			state sending {
				entry action downlink {
					send new ToolExecution(toolName = "solver", uri = "file:///solver") via link;
				}
			}
		}
	}

	part def Station {
		port dish : Link;
		attribute tool : String = "";

		exhibit state listen {
			entry; then standingBy;

			state standingBy;

			state logged;

			transition first standingBy
				accept t : ToolExecution via dish
				do assign tool := t.toolName
				then logged;
		}
	}

	part def Site {
		part probe : Probe;
		part station : Station;

		connect probe.link to station.dish;
	}

	part site : Site;
}
