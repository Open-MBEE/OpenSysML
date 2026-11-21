library package QVTS {
	doc /*
	 * NON-NORMATIVE Query/View/Transformation vocabulary for SysML v2; not part
	 * of the OMG SysML or KerML standard library.
	 * The QVTS-R library, first cut: the shape of a query result. A query is a
	 * calc def specializing DocumentQueries::Query; its result is a view that
	 * exposes the selected elements and states how it was obtained.
	 */
	private import ScalarValues::*;

	enum def Outcome {
		doc /* established: a value, possibly empty; pathError: the query path names
		     something the model does not have; modelError: the model violates a
		     constraint a step relied on. */
		established;
		pathError;
		modelError;
	}

	view def QueryResult {
		doc /* One query run. The exposed members are the elements the query selected;
		     the attributes say which query, over which source, with which outcome,
		     and where the PROV record of the run is. */
		attribute query : String;
		attribute source : String;
		attribute outcome : Outcome;
		attribute provenance : String;
	}
}
