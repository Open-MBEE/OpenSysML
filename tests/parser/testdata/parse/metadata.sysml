part def AnnotatedPart {
	attribute data[1..*] ordered {
		@Metadata;
	}
	attribute tagged {
		@Metadata { name = "tagged"; }
	}
}
