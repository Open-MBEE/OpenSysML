// The perform-actions swimlanes of SysML v2 §8.2.3.17: the example of the
// clause's table, and the shapes beside it a lane cannot hold.
package SwimLanes {
	private import StandardViewDefinitions::*;

	part def PartDef0;
	part def PartDef1;
	part def PartDef2;

	part part0 : PartDef0 {
		perform action0;
		part part1 : PartDef1 {
			perform action0.action1;
			perform action0.action4;
		}
		part part2 : PartDef2 {
			perform action0.action2;
			perform action0.action3;
		}
	}

	action action0 {
		action action1;
		action action2;
		action action3;
		action action4;
		first start then action1;
		first action1 then action2;
		first action2 then action3;
		first action3 then action4;
		first action4 then done;
	}

	// A second performer of action0 and of one of its nodes.
	part backup {
		perform action0;
		part standby {
			perform action0.action4;
		}
	}

	view performedView : ActionFlowView {
		expose part0::action0;
	}
	view actionView : ActionFlowView {
		expose action0;
	}
}

package LaneShapes {
	private import StandardViewDefinitions::*;

	action assemble {
		action fetch;
		action build {
			action cut;
			then action weld;
		}
		action inspect;
		action pack;
		first start then fetch;
		first fetch then build;
		first build then inspect;
		first inspect then pack;
	}
	part def Robot {
		perform assemble.build.cut;
		perform assemble.build.weld;
	}
	part def Cell {
		part loader {
			perform assemble.fetch;
		}
		part : Robot {
			perform assemble.pack;
		}
		action supervise {
			perform assemble.inspect;
		}
	}
	part bench {
		perform assemble.fetch;
	}

	view assembleView : ActionFlowView {
		expose assemble;
	}
}
