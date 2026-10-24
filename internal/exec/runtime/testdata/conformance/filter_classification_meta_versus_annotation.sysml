package test {
	metadata def Safety;
	metadata def Other;

	#Safety part def Belt;
	part belt : Belt;

	// `@` and `@@` both classify the subject's metadata: metadata stated about it
	// and its own metaclass hold, another metadata type and another metaclass do not.
	constraint metaVersusAnnotation {
		Belt @ Safety and Belt @@ Safety and not (Belt @@ Other)
			and Belt @@ SysML::PartDefinition and belt @@ SysML::PartUsage
			and not (belt @@ SysML::PartDefinition)
	}
}
