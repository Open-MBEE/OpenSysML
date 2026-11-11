package AnswerResult {
	private import QVTS::*;

	view answer : QueryResult {
		expose Lander::lander::cabin;
		expose Lander::lander::engine;
		expose Lander::lander::scienceBay;
		attribute :>> query = "Project(WhereType(Descendants(Named(\"Lander::lander\")), \"PartUsage\", true), (\"qualifiedName\", \"mass\"))";
		attribute :>> source = "examples/qvts/lander.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "sha256:8afb8c8cac1d730f946612f6face4667b32a0e0ef1feeb0c44d44ed6d924e206";
	}
}
