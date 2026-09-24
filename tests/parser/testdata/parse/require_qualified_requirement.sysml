// An objective may require a requirement named by a qualified reference, whose
// body redefines a qualified feature of it (SysML v2 8.2.2.18, 8.2.2.19.2).
package QualifiedRequire {
    requirement def MaxFuelMassRequirement {
        attribute actualFuelMass;
    }

    package FuelMassAnalysis {
        requirement fuelMassRequirement : MaxFuelMassRequirement;
    }

    analysis def FuelMassAnalysisCase {
        attribute calculatedFuelMass;

        objective fuelMassAnalysisObjective {
            require FuelMassAnalysis::fuelMassRequirement {
                :>> MaxFuelMassRequirement::actualFuelMass = calculatedFuelMass;
            }
            // Body-less and specialized forms are references too
            // (RequirementConstraintUsage's CalculationBody may be `;`).
            assume FuelMassAnalysis::fuelMassRequirement;
            require FuelMassAnalysis::fuelMassRequirement : MaxFuelMassRequirement;
        }
    }
}
