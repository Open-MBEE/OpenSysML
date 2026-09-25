requirement def SystemRequirement {
	doc /* Primary system requirement */
	
	subject vehicle : Vehicle;
	actor driver : Person;
	
	// nested requirement
	requirement subreq {
		subject subj : Engine;
	}
	
	// general feature member (Phase B2 fallback test)
	attribute verificationStatus : String;
	part testData : TestData;
}
