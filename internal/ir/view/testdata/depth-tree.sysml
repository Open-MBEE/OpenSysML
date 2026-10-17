package DepthViews {
	private import Views::*;

	// Four levels of decomposition under one exposed root.
	part vehicle {
		attribute mass;
		part wheel {
			attribute pressure;
			part hub {
				part bearing {
					attribute grade;
				}
			}
		}
		part engine {
			part piston {
				part ring;
			}
			port fuelIn;
		}
	}

	// A bare depth attribute, as SysIDE reads it: one level below the root.
	view depth1 {
		expose vehicle;
		attribute depth = 1;
		render asTreeDiagram;
	}

	// A typed depth a view definition declares, redefined by a specializing
	// definition and inherited by the view.
	view def DepthLimited {
		attribute depth : ScalarValues::Natural[0..1];
	}

	view def TwoLevels :> DepthLimited {
		:>> depth = 2;
	}

	view depth2 : TwoLevels {
		expose vehicle;
		render asTreeDiagram;
	}

	// The view's own depth wins over the definition's.
	view depth3 : TwoLevels {
		expose vehicle;
		:>> depth = 3;
		render asTreeDiagram;
	}

	// 0 is no bound at all.
	view unlimited : TwoLevels {
		expose vehicle;
		:>> depth = 0;
		render asTreeDiagram;
	}
}
