package AliasInTypeBody {
	package SC {
		step s1;
	}
	behavior ST {
		step trns {
			alias s1 for SC::s1;
			private alias <t> target for SC::s1 {
				doc /* An alias member of a step body. */
			}
			in feature x;
		}
	}
}
