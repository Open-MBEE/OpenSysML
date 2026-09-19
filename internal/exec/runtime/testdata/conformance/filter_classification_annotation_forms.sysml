package test {
	metadata def Safety {
		attribute level;
	}
	metadata def CrashSafety :> Safety;
	metadata def Comfort;

	#Safety part def Belt;
	part seatBelt { @Safety{level = 3;} }
	part airBag { metadata crash : CrashSafety { level = 5; } }
	part radio;
	metadata comfort : Comfort about radio;
	part keyless;

	// `@` holds for metadata annotating the subject, written in any of the forms
	// the notation allows, and for an annotation whose type specializes Safety.
	constraint annotationForms {
		Belt @ Safety and seatBelt @ Safety and airBag @ Safety
			and radio @ Comfort and not (keyless @ Safety) and not (radio @ Safety)
	}
}
