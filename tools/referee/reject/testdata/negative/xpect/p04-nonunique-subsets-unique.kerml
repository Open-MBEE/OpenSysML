// Invalid: a nonunique feature cannot subset a unique one (KerML 8.3.3.3 validateSubsettingUniquenessConformance; pilot Feature_nonunique_invalid.kerml.xt).
package P04NonuniqueSubsetsUnique {
    class C {
        feature a : C [*];
        feature b : C [*] nonunique subsets a;
    }
}
