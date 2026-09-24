package P {
	metadata def Tag { attribute id : String; }
	constraint def Positive {
		@Tag { id = "a"; }
		in x : Real;
		x > 0
	}
	constraint c : Positive {
		@Tag;
	}
	constraint named { @ m : Tag { id = "m"; } }
	constraint short { @ <t> : Tag; }
	constraint typed { @ t typed by Tag; }
	constraint tagged { @Tag }
	constraint tagged2 { @Tag and true }
	constraint tagged3 { @ Tag::sub }
	constraint less { @Tag < x; }
	constraint greater { @Tag > x; }
}
