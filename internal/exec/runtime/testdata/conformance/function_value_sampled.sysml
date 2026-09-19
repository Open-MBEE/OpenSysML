package test {
	private import ScalarValues::*;
	private import SampledFunctions::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def SquaresOf {
		attribute sampled : SampledFunction = Sample(Sq, (1.0, 2.0, 3.0));
		return : Real[0..*] = Range(sampled);
	}
}
