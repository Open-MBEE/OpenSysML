package Scalars {
    private import ScalarValues::*;

    attribute n : Integer = 4;
    attribute w : Real = 4.0;
    attribute r : Real = 2.5;
    attribute q : Rational = 6 / 3;
    attribute rat : Rational = 4;
    attribute nat : Natural = 7;

    attribute intHasRational = n hastype Rational;
    attribute intIsRational = n istype Rational;
    attribute wholeIsInteger = w istype Integer;
    attribute wholeHasInteger = w hastype Integer;
    attribute wholeHasRational = w hastype Rational;
    attribute wholeHasReal = w hastype Real;
    attribute realIsRational = r istype Rational;
    attribute realHasRational = r hastype Rational;
    attribute realHasReal = r hastype Real;
    attribute quotientIsInteger = q istype Integer;
    attribute quotientIsRational = q istype Rational;
    attribute quotientHasRational = q hastype Rational;
    attribute intDivHasRational = (7 / 2) hastype Rational;
    attribute intDivHasReal = (7 / 2) hastype Real;
    attribute realDivHasRational = (1.0 / 3.0) hastype Rational;
    attribute ratIsInteger = rat istype Integer;
    attribute ratHasInteger = rat hastype Integer;
    attribute ratHasRational = rat hastype Rational;
    attribute natIsNatural = nat istype Natural;
    attribute natHasNatural = nat hastype Natural;
    attribute natHasInteger = nat hastype Integer;
    attribute infIsInteger = (*) istype Integer;
    attribute infHasPositive = (*) hastype Positive;
    attribute infHasNatural = (*) hastype Natural;
    attribute noneIsInteger = () istype Integer;
    attribute intAsRational = n as Rational;
    attribute realAsReal = r as Real;
}
