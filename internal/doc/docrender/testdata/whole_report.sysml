package Whole {
	private import DocumentQueries::*;

	part def Telescope {
		part mount;
	}
	part telescope : Telescope;

	part def Notes :> Document {
		:>> title = "Notes";

		rep language "markdown" /* # Shared notes
		 *
		 * The report's [budget](sysml:WholeReport::mass_budget) is kept there.
		 */
	}

	part def WholeReport :> Document {
		:>> title = "Whole Report";

		rep language "markdown" /* A report whose outline is one body; see [](sysml:mass_budget),
		 * [](sysml:Notes::shared_notes) and [](sysml:appendix).
		 *
		 * # Overview
		 *
		 * The mount is [here](sysml:telescope.mount).
		 *
		 * ## Mass budget
		 *
		 * - mirror
		 * - mount
		 *
		 * Mass budget
		 * -----------
		 *
		 * The second heading of that title is `mass_budget_2`: [](sysml:mass_budget_2).
		 *
		 * ### Margins
		 *
		 * Margins close the overview.
		 *
		 * # Design
		 *
		 * Design text.
		 */

		part appendix : Section {
			:>> title = "Appendix";

			part lead : Paragraph {
				:>> text = "A part-born section after the body.";
			}

			rep language "markdown" /* ## Derivation
			 *
			 * Nested under the appendix; back to [](sysml:overview).
			 */
		}
	}
}
