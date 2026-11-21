package DoubleMassValueResult {
	private import QVTS::*;

	view doubleMassValue : QueryResult {
		expose Duplicates::doubleMass;
		attribute :>> query = "QVTSDemo::DoubleMassValue";
		attribute :>> source = "examples/qvts/lander.sysml examples/qvts/duplicates.sysml examples/qvts/queries.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "sha256:df6f9af4c0fcb42e8b57c31e1b9c5d2c7ddf56890bd556334fd1db776ea24dad";
	}
}
