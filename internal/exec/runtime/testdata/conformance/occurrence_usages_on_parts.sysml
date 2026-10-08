package OccurrenceUsagesOnParts {
	occurrence def C {
		attribute level : ScalarValues::Integer := 7;
	}

	part def Holder {
		occurrence o : C;
		ref occurrence r : C = o;
	}
}
