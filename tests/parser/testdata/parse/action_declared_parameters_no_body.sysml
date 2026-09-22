action def Sampling {
	first start then gain;
	action gain { out result : Real = 2.5; }
	first gain then measure;
	action measure {
		in gain : Real;
		in mode;
		out reading : Real[0..1];
		out frames : Integer[0..*];
	}
	first measure then done;
	flow gain.result to measure.gain;
	flow measure.reading to record.value;
	allocate measure to probe;
}
