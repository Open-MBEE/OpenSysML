package Whole {
    private import DocumentQueries::*;

    part def Telescope {
        part mirror;
        part mount;
    }
    part telescope : Telescope;

    part def MassReport :> Document {
        :>> title = "Telescope Mass Report";

        rep language "markdown" /* This report is one Markdown body: its headings are the
         * document's sections. See [](sysml:mass_budget) and the
         * [appendix](sysml:appendix) that follows the body.
         *
         * # Overview
         *
         * The telescope is the [mirror](sysml:telescope.mirror) on its
         * [mount](sysml:telescope.mount).
         *
         * ## Mass budget
         *
         * - mirror: 12 kg
         * - mount: 30 kg
         *
         * Mass budget
         * -----------
         *
         * A second heading of the same title is `mass_budget_2`: [](sysml:mass_budget_2).
         *
         * ### Margins
         *
         * Margins close the overview; the next heading returns to the top level.
         *
         * # Design
         *
         * Design notes.
         */

        part appendix : Section {
            :>> title = "Appendix";

            part lead : Paragraph {
                :>> text = "A part-born section after the body is its sibling, in declaration order.";
            }

            rep language "markdown" /* ## Derivation
             *
             * Nested one level below the appendix; back to [](sysml:overview).
             */
        }
    }
}
