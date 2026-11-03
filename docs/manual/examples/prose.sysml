package Prose {
	private import DocumentQueries::*;

	part def Telescope {
		part mount;
	}
	part telescope : Telescope;

	part def MassReport :> Document {
		:>> title = "Telescope Mass Report";

		rep language "markdown" /* This report is *generated* by `sysml -render-document`
		 * ([OpenSysML](https://opensysml.org/)); see [](sysml:breakdown) for the
		 * masses, and the mount at [](sysml:telescope.mount). Mass scales as
		 * $m \propto D^{2.5}$.
		 *
		 * - Masses are rolled up from the parts.
		 * - Margins are computed against the allocation.
		 */

		part breakdown : Section {
			:>> title = "Subsystem Masses";
			part reading : Paragraph {
				rep language "markdown" /* A **named** paragraph keeps its name as a `Ref` target. */
			}
		}
	}
}
