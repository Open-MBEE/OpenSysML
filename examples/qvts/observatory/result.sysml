package HeavyPartsResult {
	doc /* The result of a query over Observatory as a view: references to the
	     selected elements, never copies of them. */
	private import QVTS::*;

	view heavyParts : QueryResult {
		expose Observatory::telescope::mountControl;
		expose Observatory::telescope::primaryMirror;
		attribute :>> query = "QVTSExamples::HeavyParts threshold=10.0";
		attribute :>> source = "examples/observatory.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "urn:prov:bundle:heavyParts";
	}
}
