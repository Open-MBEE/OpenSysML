// A quantity written in MeasurementReferences::one multiplies and divides as the
// identity of the unit product: the product it enters folds to the derived unit
// the same product without it reaches, and holds a real reference to that unit.
// Units of dimension one that are not the identity (rad; a percent scaling one)
// stay in the product.
package test {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;
    private import MeasurementReferences::*;
    attribute <'%'> percent : DimensionOneUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = one; :>> conversionFactor = 0.01; } }
    part def Gen {
        attribute power : PowerValue;
        attribute efficiency : DimensionOneValue;
        calc deliveredEnergy { in duration : DurationValue; return : EnergyValue = power * duration * efficiency; }
    }
    part def Plant {
        part rated : Gen { attribute :>> power = 800.0 [W]; attribute :>> efficiency = 0.7 [one]; }
        part bare : Gen { attribute :>> power = 800.0 [W]; attribute :>> efficiency = 0.7; }
        attribute delivered : EnergyValue = rated.deliveredEnergy(120.0 [s]);
        attribute deliveredBare : EnergyValue = bare.deliveredEnergy(120.0 [s]);
        attribute nonNegative : Boolean = delivered >= 0.0 [SI::J];
        attribute inJoules : Boolean = delivered.mRef == J;
        attribute usable : PowerValue = rated.power * rated.efficiency;
        attribute leading : PowerValue = rated.efficiency * rated.power;
        attribute perOne : PowerValue = rated.power / 0.5 [one];
        attribute squared : DimensionOneValue = 0.5 [one] * 0.5 [one];
        attribute swept = 3 [rad] * 1 [one];
        attribute share = 50 ['%'] * 800.0 [W];
    }
}
