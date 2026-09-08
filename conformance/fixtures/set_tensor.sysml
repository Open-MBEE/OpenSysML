package T {
	private import ScalarValues::*;
	private import Collections::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
	attribute cubeRef : TensorMeasurementReference { :>> dimensions = (2, 2, 2); :>> mRefs = (m, m, m, m, m, m, m, m); }
	attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
	calc def Members { in c : Integer[0..*]; return : Integer = SequenceFunctions::size(c); }
	calc members : Members;
	calc def Corner { in t : TensorQuantityValue; return : ScalarQuantityValue = t#(2, 1, 2); }
	calc corner : Corner;
}
