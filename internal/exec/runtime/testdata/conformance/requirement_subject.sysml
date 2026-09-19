package test {
	attribute speed : Real = 50.0;
	
	requirement SafeSpeed {
		subject subj = speed;
		require constraint { subj < 100.0 }
	}
}
