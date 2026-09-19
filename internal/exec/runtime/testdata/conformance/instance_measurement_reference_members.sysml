// A measurement reference that names a declaration answers that declaration's
// members from its materialized object: `SI::km`'s unitConversion is a
// ConversionByPrefix whose conversionFactor follows the redefinition
// `= prefix.conversionFactor` and whose isExact follows UnitConversion's
// `default true`; `SI::m`'s quantityDimension and unitPowerFactors read through
// LengthUnit and SimpleUnit (`unit = self`, `exponent = 1`); a unit's
// definitionalQuantityValues are quantities. Members the value itself carries
// (isBound, mRefs, isOrthogonal, dimensions) are answered by the value, and a
// model-owned TensorMeasurementReference stating no isBound inherits the
// library's `default false`. A reference read from a quantity's mRef names its
// declaration too, so a conversion can follow the declared reference unit. An
// optional record whose body binds a feature only at depth (`:>> conv { :>>
// prefix { :>> conversionFactor = 1000.0; } }`) is one object, not none; one whose only
// binding sits under an abstract member stays none, as the abstract member holds nothing,
// and so does one whose body only values a calc's parameters, which bind no field. A field
// the model adds to a TensorMeasurementReference (`label`) is state of its own and reads;
// a constraint the model adds holds no value, so a usage stating none stays unset.
package test {
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import QuantityCalculations::*;
	private import ScalarValues::*;

	attribute stressRef : TensorMeasurementReference {
		:>> mRefs = (Pa, Pa, Pa, Pa);
		:>> dimensions = (2, 2);
	}

	attribute def Box { attribute conv : ConversionByPrefix[0..1]; }
	attribute deep : Box { :>> conv { :>> prefix { :>> conversionFactor = 1000.0; } } }
	attribute hollow : Box { :>> conv { abstract attribute extra : UnitPrefix { :>> conversionFactor = 1000.0; } } }
	attribute computing : Box { :>> conv { calc scaled { in x : Real = 1000.0; return : Real = x; } } }
	attribute def LabeledRef :> TensorMeasurementReference { attribute label : String = "stress"; }
	attribute labeled : LabeledRef;
	attribute def CheckedRef :> TensorMeasurementReference { constraint bounded { isBound == false } }

	part def Survey {
		attribute side : LengthValue = 3 [km];
		attribute factor : Real = SI::km.unitConversion.conversionFactor;
		attribute reference : LengthUnit = SI::km.unitConversion.referenceUnit;
		attribute exact : Boolean = SI::km.unitConversion.isExact;
		attribute prefixName : String = SI::km.unitConversion.prefix.longName;
		attribute fromSide : Real = side.mRef.unitConversion.conversionFactor;
		attribute inReference : LengthValue = ConvertQuantity(side, km.unitConversion.referenceUnit);
		attribute exponent : Real = SI::m.quantityDimension.quantityPowerFactors#(1).exponent;
		attribute derivedExponent : Real = SI::'m/s'.quantityDimension.quantityPowerFactors#(2).exponent;
		attribute factorUnit : MeasurementUnit = SI::m.unitPowerFactors#(1).unit;
		attribute factorExponent : Real = SI::m.unitPowerFactors#(1).exponent;
		attribute triplePoint : Real = SI::K.definitionalQuantityValues#(1).num;
		attribute bound : Boolean = SI::m.isBound;
		attribute orthogonal : Boolean = SI::m.isOrthogonal;
		attribute axes : MeasurementUnit[*] = SI::m.mRefs;
		attribute noConversion : UnitConversion[0..1] = SI::m.unitConversion;
		attribute stressBound : Boolean = stressRef.isBound;
		attribute deepPrefixFactor : Real = deep.conv.prefix.conversionFactor;
		attribute deepFactor : Real = deep.conv.conversionFactor;
		attribute hollowConv : ConversionByPrefix[0..1] = hollow.conv;
		attribute computingConv : ConversionByPrefix[0..1] = computing.conv;
		attribute label : String = labeled.label;
		attribute labeledBound : Boolean = labeled.isBound;
		attribute checked : CheckedRef;
	}
}
