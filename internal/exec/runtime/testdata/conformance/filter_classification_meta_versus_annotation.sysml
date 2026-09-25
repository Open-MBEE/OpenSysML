package test {
	metadata def Safety;

	#Safety part def Belt;
	part belt : Belt;

	// `@` holds for metadata stated about the subject and for the subject's own
	// metaclass; `@@` holds for the metaclass alone.
	constraint metaVersusAnnotation {
		Belt @ Safety and not (Belt @@ Safety)
			and Belt @@ SysML::PartDefinition and belt @@ SysML::PartUsage
			and not (belt @@ SysML::PartDefinition)
	}
}
