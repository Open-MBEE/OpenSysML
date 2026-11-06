// Usage::owningUsage and Usage::owningDefinition derive a usage's owning type
// when it is a usage, respectively a definition (SysML v2 §8.3.6.4): occ1 is
// owned by the definition Occ, occ2 by the usage occ1, and a usage a package
// owns has neither. `usage` and `directedUsage` are the usages among a type's
// features.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def A;
    occurrence def Occ {
        occurrence occ1 {
            occurrence occ2;
            assert constraint c1 { (occ1 meta SysML::OccurrenceUsage).owningDefinition->size() == 1 }
        }
    }
    occurrence top;
    action def Act { in x : A; out y : A; part p : A; }

    part def Report {
        attribute occ1OwningUsages = (Occ::occ1 meta SysML::OccurrenceUsage).owningUsage->size();
        attribute occ1OwningDefinitions = (Occ::occ1 meta SysML::OccurrenceUsage).owningDefinition->size();
        attribute occ2OwningUsages = (Occ::occ1::occ2 meta SysML::OccurrenceUsage).owningUsage->size();
        attribute occ2OwningDefinitions = (Occ::occ1::occ2 meta SysML::OccurrenceUsage).owningDefinition->size();
        attribute topOwningUsages = (top meta SysML::OccurrenceUsage).owningUsage->size();
        attribute topOwningDefinitions = (top meta SysML::OccurrenceUsage).owningDefinition->size();
        attribute occ1Usages = (Occ::occ1 meta SysML::OccurrenceUsage).usage->size();
        attribute actDirectedUsages = (Act meta SysML::ActionDefinition).directedUsage->size();
    }
}
