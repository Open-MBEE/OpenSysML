// SamplePair::domainValue and rangeValue redefine KeyValuePair's `[0..*]` key
// and val without a multiplicity, so each is a one-element sequence denoting
// its element in arithmetic; interpolateLinear reads them the same way, and its
// result is what the library's own Linear formula computes.
package test {
	private import ScalarValues::*;
	private import SampledFunctions::*;

	calc def Pairs {
		attribute s : SampledFunction = new SampledFunction(
			samples = (new SamplePair(1.0, 2.0), new SamplePair(3.0, 4.0)));
		out difference = s.samples#(2).domainValue - s.samples#(1).domainValue;
		out product = s.samples#(1).rangeValue * s.samples#(2).rangeValue;
		out interpolated = interpolateLinear(s, 2.0);
	}

	calc c : Pairs;
}
