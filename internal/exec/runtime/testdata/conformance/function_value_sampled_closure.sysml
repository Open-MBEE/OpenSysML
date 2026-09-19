package test {
	private import ScalarValues::*;
	private import SampledFunctions::*;
	part def Scaler {
		attribute k : Real = 10.0;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part scaler : Scaler;
	calc def ScaledDomain {
		attribute sampled : SampledFunction = Sample(calculation = scaler.scale, domainValues = (1.0, 2.0));
		return : Real[0..*] = (Domain(sampled), Range(sampled));
	}
}
