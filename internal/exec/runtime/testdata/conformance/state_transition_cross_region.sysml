// A guardless succession into a vertex of a sibling orthogonal region exits its
// source only: KerML StatePerformances::StateTransitionPerformance orders
// `guard then transitionLinkSource.exit`, so the enclosing composite state is
// neither exited nor re-entered and the succession cannot re-enable itself.
package Test {
	state def TransitionCrossRegion {
		attribute crossed : Integer = 0;
		attribute reentered : Integer = 0;

		entry; then start;
		state start;
		state running parallel {
			entry { assign reentered := reentered + 1; }

			state left {
				entry; then lstart;
				state lstart;
				state lidle;

				succession first lstart then lidle;
				transition first lidle then rtarget;
			}
			state right {
				entry; then rstart;
				state rstart;
				state ridle;
				state rtarget { entry { assign crossed := crossed + 1; } }

				succession first rstart then ridle;
			}
		}

		succession first start then running;
	}
}
