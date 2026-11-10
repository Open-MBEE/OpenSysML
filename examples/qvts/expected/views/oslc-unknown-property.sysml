package AnswerResult {
	private import QVTS::*;

	view answer : QueryResult {
		attribute :>> query = "oslc.where=sysml:nme=\"cabin\"";
		attribute :>> source = "examples/qvts/lander.sysml";
		attribute :>> outcome = Outcome::pathError;
		attribute :>> provenance = "sha256:8afb8c8cac1d730f946612f6face4667b32a0e0ef1feeb0c44d44ed6d924e206; step Feature(sysml:nme)";
	}
}
