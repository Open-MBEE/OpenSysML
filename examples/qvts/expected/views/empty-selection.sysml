package EmptySelectionResult {
	private import QVTS::*;

	view emptySelection : QueryResult {
		attribute :>> query = "QVTSDemo::EmptySelection";
		attribute :>> source = "examples/qvts/lander.sysml examples/qvts/queries.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "sha256:110e03c52250fab7b72729f2c70e2cf09f2d1777290be6e30270b523b26e12dd";
	}
}
