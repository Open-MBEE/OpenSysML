// Census: validateSubsettingConstantConformance
// Expect: error: Subsetting/redefining feature must be constant if subsetted/redefined feature is constant
package K14VariableSubsetsConstant {
    class C specializes Occurrences::Occurrence {
        const feature a : C;
        var feature b : C subsets a;
    }
}
