package F50 {
	classifier C;
	classifier D {
		abstract var feature x [0..*];
		member abstract feature y [0..*] featured by C {
			doc /* body */
		}
	}
}
