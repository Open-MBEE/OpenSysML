// A feature value belongs to a feature, not to a name (KerML 1.0 §7.3.4.5): a
// redefinition makes the redefining and the redefined feature read one value,
// while a namesake inherited along another path, which nothing redefines, keeps
// its own. `Both :> A2, A1 { attribute b :>> A1::a = 3.0; }` reaches two features
// named `a`; the one an object of Both has under `a` is the surviving `A2::a`
// (§8.3.3.1.10 removeRedefinedFeatures removes only `A1::a`), read as 2.0
// whichever order the generals are written in and whether a definition or a usage
// owns the redefinition, while `b`, and `a` read as an A1, are 3.0. Where the two
// paths inherit one feature (a diamond) every name reads the redefiner's value.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::size;

    part def A1 { attribute a : Real default = 1.0; }
    part def A2 { attribute a : Real default = 2.0; }
    part def Both21 :> A2, A1 { attribute b :>> A1::a = 3.0; }
    part def Both12 :> A1, A2 { attribute b :>> A1::a = 3.0; }
    part bd21 : Both21;
    part bd12 : Both12;
    part both : A2, A1 { attribute b :>> A1::a = 3.0; }

    part def A0 { attribute a : Real default = 1.0; }
    part def D1 :> A0;
    part def D2 :> A0;
    part def Diamond :> D2, D1 { attribute b :>> D1::a = 3.0; }
    part dia : Diamond;

    part reader {
        attribute a21 : Real = bd21.a;
        attribute b21 : Real = bd21.b;
        attribute a21Count : Integer = bd21.a->size();
        ref bd21AsA1 : A1 = bd21;
        ref bd21AsA2 : A2 = bd21;
        attribute a21ViaA1 : Real = bd21AsA1.a;
        attribute a21ViaA2 : Real = bd21AsA2.a;
        attribute a21CastA1 : Real = (bd21 as A1).a;
        attribute a21CastA2 : Real = (bd21 as A2).a;
        attribute a12 : Real = bd12.a;
        attribute b12 : Real = bd12.b;
        attribute a12CastA1 : Real = (bd12 as A1).a;
        attribute a12CastA2 : Real = (bd12 as A2).a;
        attribute aUsage : Real = both.a;
        attribute bUsage : Real = both.b;
        attribute aUsageCastA1 : Real = (both as A1).a;
        attribute aUsageCastA2 : Real = (both as A2).a;
        attribute bDiamond : Real = dia.b;
        attribute aDiamondCastD1 : Real = (dia as D1).a;
        attribute aDiamondCastD2 : Real = (dia as D2).a;
    }
}
