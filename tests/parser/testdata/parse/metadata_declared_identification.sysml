package DeclaredMetadata {
	metadata def Security;

	part def A {
		@Security;
		@ m1 : Security;
		@ <sn> m2 : Security;
		@ : Security;
		@ m3 typed by Security;
		@ m4 : Security { classification = "conf"; }
	}
}
