// Invalid: a usage's types are Classifiers, nominally definitions, and a feature chain ends in a feature (SysML 8.3.6.4 Usage::definition redefines type as Classifier, 7.6.1; pilot validateUsageType, validateOccurrenceUsageType).
package S117UsageTypedByFeatureChain {
    part def Rig {
        part wheel : Wheel;
    }
    part def Wheel;
    part rig : Rig;
    ref spare : rig.wheel;
    part mounted : rig.wheel;
}
