// Invalid: a variable feature cannot subset a constant one unless it is constant (KerML 8.3.3.3 validateSubsettingConstantConformance; pilot validateSubsettingConstantConformance).
package K14VariableSubsetsConstant {
    class C specializes Occurrences::Occurrence {
        const feature a : C;
        var feature b : C subsets a;
    }
}
