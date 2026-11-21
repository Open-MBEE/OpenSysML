package AnswerResult {
	private import QVTS::*;

	view answer : QueryResult {
		expose Lander::lander::cabin;
		attribute :>> query = "oslc.where=sysml:name=\"cabin\"";
		attribute :>> source = "examples/qvts/lander.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "sha256:8afb8c8cac1d730f946612f6face4667b32a0e0ef1feeb0c44d44ed6d924e206";
	}
}
