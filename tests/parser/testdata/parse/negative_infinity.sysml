package test {
    private import ScalarValues::*;

    part def Limits {
        attribute lowest : Real = -*;
        attribute restored = -(-*);
        attribute belowFinite = -* < -1000;
        attribute sum = * + 1;
        attribute decay = 2 ** -*;
        attribute least = RealFunctions::min(-*, 3.0);
    }
}
