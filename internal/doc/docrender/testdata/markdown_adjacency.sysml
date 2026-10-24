package Prose {
	private import DocumentQueries::*;

	part def AdjacencyReport :> Document {
		:>> title = "Adjacency";
		rep language "markdown" /* a*bold*b, c`code`d, [x](https://example.com/x)y and $m$! Then
		 * [](sysml:reading)'s turn.
		 *
		 * - see [](sysml:reading)
		 * - and [](sysml:closing)
		 */
		part reading : Paragraph {
			rep language "markdown" /* Reading. */
		}
		part closing : Paragraph {
			:>> text = "Closing.";
		}
	}
}
