package test {
    private import ScalarValues::*;
    private import OccurrenceFunctions::*;

    part def Target {}

    action destroyTarget {
        in target : Target = new Target();

        out result : Target[0..1] = OccurrenceFunctions::destroy(target);
        out alive : Boolean = OccurrenceFunctions::isDuring(result);
    }
}
