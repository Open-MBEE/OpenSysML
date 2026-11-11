package MissingMassProjectResult {
	private import QVTS::*;

	view missingMassProject : QueryResult {
		expose Lander::lander::cabinEngine;
		expose Lander::lander::engineBay;
		attribute :>> query = "QVTSDemo::MissingMassProject";
		attribute :>> source = "examples/qvts/lander.sysml examples/qvts/queries.sysml";
		attribute :>> outcome = Outcome::pathError;
		attribute :>> provenance = "sha256:110e03c52250fab7b72729f2c70e2cf09f2d1777290be6e30270b523b26e12dd; step Feature(mass)";
	}
}
