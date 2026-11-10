package AmbiguousPartResult {
	private import QVTS::*;

	view ambiguousPart : QueryResult {
		expose Duplicates;
		attribute :>> query = "QVTSDemo::AmbiguousPart";
		attribute :>> source = "examples/qvts/lander.sysml examples/qvts/duplicates.sysml examples/qvts/queries.sysml";
		attribute :>> outcome = Outcome::modelError;
		attribute :>> provenance = "sha256:df6f9af4c0fcb42e8b57c31e1b9c5d2c7ddf56890bd556334fd1db776ea24dad; step Named(Duplicates::repeated)";
	}
}
