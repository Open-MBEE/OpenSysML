// The relationship glyphs of the graphical notation that are drawn as edges
// of their own: a succession flow, a required, assumed or asserted constraint,
// a framed concern, an event occurrence, a timeslice and a snapshot, a
// reference subsetting, an alias membership and a view's exposure.
package NotationGlyphs {
	private import ScalarValues::*;
	private import StandardViewDefinitions::*;
	private import Views::*;
	private import OpenSysMLRenderings::*;

	part def Tank {
		attribute level : Real;
	}
	part def Pump;
	part def Plant {
		part tank : Tank;
		part pump : Pump;
		timeslice filling;
		snapshot full;
		event occurrence startup;
		ref part spare ::> tank;
	}
	constraint def Bounded {
		in x : Real;
	}
	constraint bounded : Bounded;
	constraint tight : Bounded;
	concern safety {
		doc /* Nothing overflows. */
	}
	requirement def LevelReq {
		subject plant : Plant;
		require constraint keepLevel { plant.tank.level >= 0.0 }
		assume constraint lowLevel { plant.tank.level <= 100.0 }
		frame safety;
		require bounded;
		assume tight;
	}
	requirement levelReq : LevelReq;
	requirement capacity {
		require levelReq;
		assert constraint asserted { true }
	}
	action def Fill {
		action open {
			out water : Real;
		}
		action pour {
			in water : Real;
		}
		succession flow open.water to pour.water;
	}
	part def Line {
		part a : Tank;
		part b : Tank;
		succession flow a.level to b.level;
	}
	part line : Line;
	package Lib {
		part def Valve;
	}
	package Uses {
		alias V for Lib::Valve;
		alias L for Lib;
	}
	package Diagrams {
		view requirementGlyphs : GeneralView {
			filter @SysML::RequirementDefinition or @SysML::RequirementUsage;
			expose LevelReq;
			expose levelReq;
			expose capacity;
			expose safety;
		}
		view definitionGlyphs : GeneralView {
			filter @SysML::Definition or @SysML::Usage;
			expose Plant;
			expose Plant::tank;
			expose Plant::spare;
			expose Plant::filling;
			expose Plant::full;
			expose Plant::startup;
			expose LevelReq;
			expose LevelReq::keepLevel;
			expose LevelReq::lowLevel;
			expose bounded;
			expose tight;
			expose capacity;
			expose capacity::asserted;
		}
		view packageGlyphs : GeneralView {
			filter @SysML::Package;
			expose NotationGlyphs;
			expose Lib;
			expose Uses;
		}
		view tankView {
			expose Tank;
		}
		view treeGlyphs {
			expose Plant;
			expose tankView;
			expose Tank;
		}
		view lineGlyphs {
			expose line;
			render asInterconnectionDiagram;
		}
		view fillGlyphs : ActionFlowView {
			expose Fill;
		}
		view mixedGlyphs {
			expose LevelReq;
			expose levelReq;
			expose capacity;
			expose bounded;
			expose tight;
			expose tankView;
			expose Tank;
			render asMixedDiagram;
		}
	}
}
