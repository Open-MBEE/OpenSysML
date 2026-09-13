// A choice whose every branch enters a sibling region leaves that region's
// active state, so its exit behavior runs before the choice's guards are read,
// as it does for a transition straight into the sibling region. The guard sees
// the write the sibling's exit made, and the branch it enables is taken.
package Test {
	state def RegionCrossChoiceSiblingExit {
		attribute x : Integer = 0;
		attribute picked : Integer = 0;

		entry; then outer;

		state outer parallel {
			state left {
				entry; then l;
				state l;

				transition first l then pick;
			}

			state right {
				entry; then r;
				state r { exit { assign x := 1; } }
				state seen { entry { assign picked := 1; } }
				state missed { entry { assign picked := 2; } }
			}
		}

		choice pick;

		transition first pick if x == 1 then seen;
		transition first pick then missed;
	}
}
